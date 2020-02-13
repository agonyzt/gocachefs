package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
)

// Provides filesystem implementation
type GoCacheFs struct {
	fuse.FileSystemBase
	Dst                  string
	Src                  string
	WriteBack            bool
	Jobs                 chan Job
	MaxWriteBackAttempts uint8
}

func (fs *GoCacheFs) Getattr(relPath string, stat *fuse.Stat_t, fh uint64) int {
	goStat := new(syscall.Stat_t)
	if isPathDeleted(relPath, fs) {
		return -fuse.ENOENT
	} else if err := syscall.Stat(toDstPath(relPath, fs), goStat); err == nil {
		copyFusestatFromGostat(stat, goStat)
		return 0
	} else if err := syscall.Stat(toSrcPath(relPath, fs), goStat); err == nil {
		copyFusestatFromGostat(stat, goStat)
		return 0
	}
	return -fuse.ENOENT
}

func (fs *GoCacheFs) Statfs(relPath string, stat *fuse.Statfs_t) int {
	goStat := new(syscall.Statfs_t)
	if err := syscall.Statfs(toSrcPath(relPath, fs), goStat); err != nil {
		return -fuse.ENOSYS
	}
	copyFusestatfsFromGostatfs(stat, goStat)
	return 0
}

func (fs *GoCacheFs) Readdir(relPath string, fill func(name string, stat *fuse.Stat_t, offset int64) bool, offset int64, fh uint64) int {
	// Get the list of files/dirs on src
	srcPath := toSrcPath(relPath, fs)
	entities, err := ioutil.ReadDir(srcPath)
	if err != nil {
		return -fuse.ENOENT
	}

	// Get the list of files/dirs on dst, which may or may not exist
	dstPath := toDstPath(relPath, fs)
	dstEntities, dstErr := ioutil.ReadDir(dstPath)
	if dstErr != nil {
		dstEntities = []os.FileInfo{}
	}

	// Get list of entities from both src and dst
	names := make(map[string]*fuse.Stat_t)
	removed := make(map[string]bool)
	for i, entity := range entities {
		if int64(i) >= offset {
			goStat, ok := entity.Sys().(*syscall.Stat_t)
			stat := new(fuse.Stat_t)
			if ok {
				copyFusestatFromGostat(stat, goStat)
				names[entity.Name()] = stat
			}
		}
	}

	// Create a list of entities found in each
	// as well as mark which ones were deleted on dst
	for _, entity := range dstEntities {
		name := entity.Name()
		if _, ok := names[name]; !ok {
			if isDeletedFilename(name) {
				removed[name] = true
				removed[toNonDeletedPath(name)] = true
			} else {
				goStat, ok := entity.Sys().(*syscall.Stat_t)
				stat := new(fuse.Stat_t)
				if ok {
					copyFusestatFromGostat(stat, goStat)
					names[name] = stat
				}
			}
		}
	}

	// Report entities back to fuse
	for name, entity := range names {
		if _, ok := removed[name]; !ok {
			fill(name, entity, 0)
		}
	}

	return 0
}

func (fs *GoCacheFs) Open(relPath string, flags int) (int, uint64) {
	if isPathDeleted(relPath, fs) {
		return -fuse.ENOENT, 0
	}

	if err := ensureCachedFile(relPath, fs); err != nil {
		return errno(err), 0
	}

	f, err := syscall.Open(toDstPath(relPath, fs), flags, 0)
	if err != nil {
		return errno(err), 0
	}

	return 0, uint64(f)
}

func (fs *GoCacheFs) Release(relPath string, fh uint64) int {
	return errno(syscall.Close(int(fh)))
}

func (fs *GoCacheFs) Read(relPath string, buff []byte, offset int64, fh uint64) int {
	numRead, err := syscall.Pread(int(fh), buff, offset)
	if err != nil {
		return errno(err)
	}

	return numRead
}

func (fs *GoCacheFs) Write(relPath string, buff []byte, offset int64, fh uint64) int {
	numWrote, err := syscall.Pwrite(int(fh), buff, offset)
	if err != nil {
		return errno(err)
	}

	go enqueueWriteBackJob(fs, relPath)

	return numWrote
}

func (fs *GoCacheFs) Truncate(relPath string, size int64, fh uint64) int {
	if err := syscall.Ftruncate(int(fh), size); err != nil {
		if err := ensureCachedFile(relPath, fs); err != nil {
			return errno(err)
		}

		if err = os.Truncate(toDstPath(relPath, fs), size); err != nil {
			return errno(err)
		}
	}

	go enqueueWriteBackJob(fs, relPath)

	return 0
}

func (fs *GoCacheFs) Create(relPath string, flags int, mode uint32) (int, uint64) {
	fd, err := syscall.Open(toDstPath(relPath, fs), flags, mode)
	if nil != err {
		return errno(err), ^uint64(0)
	}

	syscall.Unlink(toDeletedPath(relPath, fs))

	go enqueueWriteBackJob(fs, relPath)

	return 0, uint64(fd)
}

func (fs *GoCacheFs) Mkdir(relPath string, mode uint32) int {
	dstDir := toDstPath(relPath, fs)
	if err := os.MkdirAll(dstDir, os.FileMode(mode)); err != nil {
		fmt.Println("Cannot create directory", dstDir, err)
		return -fuse.EPERM
	}

	os.RemoveAll(toDeletedPath(relPath, fs))

	return 0
}

func (fs *GoCacheFs) Chmod(relPath string, mode uint32) int {
	if err := syscall.Chmod(toDstPath(relPath, fs), mode); err != nil {
		if err := syscall.Chmod(toSrcPath(relPath, fs), mode); err != nil {
			return -fuse.EPERM
		}
	}

	return 0
}

func (fs *GoCacheFs) Rm(relPath string) int {
	stat, err := os.Stat(toDstPath(relPath, fs))
	if err != nil {
		return errno(err)
	}

	if stat.IsDir() {
		return fs.Rmdir(relPath)
	}

	return fs.Unlink(relPath)
}

func (fs *GoCacheFs) Rmdir(relPath string) int {
	os.RemoveAll(toDstPath(relPath, fs))

	fh, _ := os.Create(toDeletedPath(relPath, fs))
	defer fh.Close()

	go enqueueDeleteBackJob(fs, relPath)

	return 0
}

func (fs *GoCacheFs) Unlink(relPath string) int {
	fh, err := os.Create(toDeletedPath(relPath, fs))
	defer fh.Close()
	if err != nil {
		return errno(err)
	}

	if _, err := os.Stat(toDstPath(relPath, fs)); !os.IsNotExist(err) {
		if err := syscall.Unlink(toDstPath(relPath, fs)); err != nil {
			return errno(err)
		}
	}

	go enqueueDeleteBackJob(fs, relPath)

	return 0
}

func (fs *GoCacheFs) Rename(oldPath string, newPath string) int {
	if err := ensureCachedFile(oldPath, fs); err != nil {
		return -fuse.EFAULT
	}

	if err := os.Rename(toDstPath(oldPath, fs), toDstPath(newPath, fs)); err != nil {
		return errno(err)
	}

	fh, _ := os.Create(toDeletedPath(oldPath, fs))
	defer fh.Close()

	go enqueueWriteBackJob(fs, newPath)
	go enqueueDeleteBackJob(fs, oldPath)

	return 0
}

func (fs *GoCacheFs) Utimens(relPath string, tmsp []fuse.Timespec) int {
	if err := ensureCachedFile(relPath, fs); err != nil {
		return errno(err)
	}

	times := make([]syscall.Timespec, len(tmsp))
	for i := range tmsp {
		times[i] = syscall.Timespec{Sec: tmsp[i].Sec, Nsec: tmsp[i].Nsec}
	}

	if err := syscall.UtimesNano(toDstPath(relPath, fs), times); err != nil {
		return errno(err)
	}

	return 0
}

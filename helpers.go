package main

import (
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
)

const RM_PREFIX = ".__gocachefs_rm__"

func toSrcPath(name string, fs *GoCacheFs) string {
	return path.Join(fs.Src, name)
}

func toDstPath(name string, fs *GoCacheFs) string {
	return path.Join(fs.Dst, name)
}

func toDeletedPath(relPath string, fs *GoCacheFs) string {
	name := ""
	if strings.Index(relPath, "/") == 0 {
		chars := []rune(relPath)
		name = string(chars[1:])
	}
	name = strings.ReplaceAll(relPath, string(os.PathSeparator), "----")
	strs := [2]string{RM_PREFIX, name}
	return path.Join(fs.Dst, strings.Join(strs[:], ""))
}

func isDeletedFilename(name string) bool {
	return strings.Contains(name, RM_PREFIX)
}

func isPathDeleted(path string, fs *GoCacheFs) bool {
	_, err := os.Stat(toDeletedPath(path, fs))
	if err != nil {
		return false
	}
	return true
}

func toNonDeletedPath(name string) string {
	return strings.ReplaceAll(strings.Join(strings.Split(name, RM_PREFIX), ""), "----", string(os.PathSeparator))
}

func ensureCachedFile(relPath string, fs *GoCacheFs) error {
	stat, err := os.Stat(toSrcPath(relPath, fs))
	_, err = os.Stat(toDstPath(relPath, fs))
	if os.IsExist(err) {
		return nil
	} else if os.IsNotExist(err) {
		if stat != nil && stat.IsDir() {
			return cacheDir(relPath, fs)
		}
		return cacheFile(relPath, fs)
	}
	return err
}

func cacheDir(relPath string, fs *GoCacheFs) error {
	if err := exec.Command("cp", "-r", toSrcPath(relPath, fs), toDstPath(relPath, fs)).Run(); err != nil {
		return err
	}
	return nil
}

func cacheFile(relPath string, fs *GoCacheFs) error {
	src := toSrcPath(relPath, fs)
	dst := toDstPath(relPath, fs)

	if err := os.MkdirAll(path.Dir(dst), 0140755); err != nil {
		return err
	}

	reader, err := os.Open(src)
	if err != nil {
		return err
	}

	writer, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, reader)
	return err
}

func errno(err error) int {
	if nil != err {
		return -int(err.(syscall.Errno))
	}
	return 0
}

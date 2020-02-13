// +build linux

package main

import (
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
)

func copyFusestatfsFromGostatfs(dst *fuse.Statfs_t, src *syscall.Statfs_t) {
	*dst = fuse.Statfs_t{}
	dst.Bsize = uint64(src.Bsize)
	dst.Frsize = 1
	dst.Blocks = uint64(src.Blocks)
	dst.Bfree = uint64(src.Bfree)
	dst.Bavail = uint64(src.Bavail)
	dst.Files = uint64(src.Files)
	dst.Ffree = uint64(src.Ffree)
	dst.Favail = uint64(src.Ffree)
	dst.Namemax = 255 //uint64(src.Namelen)
}

func copyFusestatFromGostat(dst *fuse.Stat_t, src *syscall.Stat_t) {
	*dst = fuse.Stat_t{}
	dst.Dev = uint64(src.Dev)
	dst.Ino = uint64(src.Ino)
	dst.Mode = uint32(src.Mode)
	dst.Nlink = uint32(src.Nlink)
	dst.Uid = uint32(src.Uid)
	dst.Gid = uint32(src.Gid)
	dst.Rdev = uint64(src.Rdev)
	dst.Size = int64(src.Size)
	dst.Atim.Sec, dst.Atim.Nsec = src.Atim.Sec, src.Atim.Nsec
	dst.Mtim.Sec, dst.Mtim.Nsec = src.Mtim.Sec, src.Mtim.Nsec
	dst.Ctim.Sec, dst.Ctim.Nsec = src.Ctim.Sec, src.Ctim.Nsec
	dst.Blksize = int64(src.Blksize)
	dst.Blocks = int64(src.Blocks)
}

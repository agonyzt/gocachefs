package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
)

func mount(src, dst, mountpoint string, debug bool, fuseOptions string, writeBack bool, jobs chan Job, maxTries uint8) bool {
	var args []string
	fs := GoCacheFs{Src: src, Dst: dst, Jobs: jobs, WriteBack: writeBack, MaxWriteBackAttempts: maxTries}
	host := fuse.NewFileSystemHost(&fs)
	if debug {
		fuseOptions = strings.Join([]string{"debug", fuseOptions}, ",")
	}
	if len(fuseOptions) > 0 {
		args = []string{"-o", fuseOptions}
	}

	return host.Mount(mountpoint, args[0:])
}

/*
This program has two main goroutines:
- the main process channel is used to handle all fuse operations
- a worker channel used to process writeback operations to the src filesystem

Additionally, a separate channel is used for each writeback operation.
*/
func main() {
	var src = flag.String("src", "", "path to the src directory")
	var dst = flag.String("dst", "", "path to the cache destination directory")
	var mountpoint = flag.String("mountpoint", "", "path to the mountpoint")
	var writeBack = flag.Bool("w", false, "writeback files from the cache to the src directory. Only use on low-latency src filesystems")
	var maxTries = flag.Int("retries", 5, "max number of writeback attempts")
	var maxConcurrent = flag.Int("concurrent", 10, "max concurrent writeback operations")
	var debug = flag.Bool("d", false, "enables debug output")
	var fuseOptions = flag.String("o", "", "fuse options")
	var logWorker = flag.Bool("log", false, "enables logging of worker operations")
	var retryDelay = flag.Int("retry-delay", 0, "the number of seconds to wait before retrying a failed writeback operation")
	var startDelay = flag.Int("start-delay", 0, "the number of seconds to wait before starting a writeback operation")
	flag.Parse()

	if len(*src) == 0 || len(*dst) == 0 || len(*mountpoint) == 0 {
		flag.Usage()
	} else {
		jobs := make(chan Job)

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGUSR1)

		settings := &WorkerSettings{
			uint8(*retryDelay),
			uint8(*startDelay),
			uint8(*maxConcurrent),
			*debug || *logWorker,
		}

		go Worker(jobs, sigs, settings)

		if !mount(*src, *dst, *mountpoint, *debug, *fuseOptions, *writeBack, jobs, uint8(*maxTries)) {
			log.Fatalf("Cannot mount")
		}
		jobs <- Job{TaskType: EOF}
	}
}

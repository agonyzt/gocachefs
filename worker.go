package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"
)

type TaskType int

const EOF = TaskType(1)
const WRITEBACK = TaskType(2)
const DELETEBACK = TaskType(3)

type Job struct {
	TaskType        TaskType
	OrigFilename    string
	UpdatedFilename string
	MaxTries        uint8
}

type WorkerSettings struct {
	retryDelay    uint8
	startDelay    uint8
	maxConcurrent uint8
	debug         bool
}

func writeBack(job Job, i uint8, done bool, delay uint8) bool {
	if done {
		return done
	}

	if i == job.MaxTries {
		return false
	}
	if err := os.MkdirAll(path.Dir(job.OrigFilename), 0140755); err == nil {
		if dst, err := os.Create(job.OrigFilename); err == nil {
			defer dst.Close()
			if src, err := os.Open(job.UpdatedFilename); err == nil {
				defer src.Close()
				if _, err := io.Copy(dst, src); err == nil {
					done = true
				}
			}
		}
	}

	if delay > 0 {
		time.Sleep(toNanoseconds(delay))
	}

	return writeBack(job, i+1, done, delay) // TCO

}

func runWriteBackJob(job Job, settings *WorkerSettings) {
	if settings.debug {
		fmt.Println("Attempting to writeback ", job.UpdatedFilename)
	}

	if writeBack(job, 1, false, settings.retryDelay) && settings.debug {
		fmt.Println("Wrote back ", job.OrigFilename)

	} else if settings.debug {
		fmt.Println("Failed to write back ", job.OrigFilename)
	}
}

func runDeleteBackJob(job Job, settings *WorkerSettings) {
	if settings.debug {
		fmt.Println("Attempting to delete on src", job.OrigFilename)
	}

	if err := os.RemoveAll(job.OrigFilename); err != nil {
		fmt.Println("Deleted src", job.OrigFilename)
	} else {
		fmt.Println("Failed to delete src", job.OrigFilename)
	}
}

func runJob(job Job, done chan string, settings *WorkerSettings) {
	switch job.TaskType {
	case WRITEBACK:
		runWriteBackJob(job, settings)
	case DELETEBACK:
		runDeleteBackJob(job, settings)
	}

	if done != nil {
		done <- job.UpdatedFilename
	}
}

func isAtMaxConcurrency(running map[string]Job, maxConcurrent uint8) bool {
	return uint8(len(running)) == maxConcurrent
}

func noop() {
	return
}

func SliceContains(s []string, e string) int {
	var i int
	i = 0
	for _, a := range s {
		if a == e {
			return i
		}
		i = i + 1
	}
	return -1
}

func toNanoseconds(second uint8) time.Duration {
	return time.Duration(1e+9 * uint64(second))
}

func Worker(jobs chan Job, sigs chan os.Signal, settings *WorkerSettings) {
	running := make(map[string]Job)
	done := make(chan string)

	// Ensure that concurrency is set to at least 1
	if settings.maxConcurrent <= 0 {
		settings.maxConcurrent = 1
	}

	// Delay starting the work queue
	if settings.startDelay > 0 {
		time.Sleep(toNanoseconds(settings.startDelay))
	}

	// Process up to max concurrent jobs
	for {
		select {
		// Check if there are jobs to process
		case job := <-jobs:
			if job.TaskType == EOF {
				// Stop processing jobs
				close(jobs)
				close(done)
				break

			} else if !isAtMaxConcurrency(running, settings.maxConcurrent) {
				// We can still process jobs
				if value, exist := running[job.UpdatedFilename]; exist {
					// Are we already processing this job?
					if settings.debug {
						fmt.Println("Already using ", value)
					}
					jobs <- job
				} else {
					// Run the job
					go runJob(job, done, settings)
					running[job.UpdatedFilename] = job
				}
			}
		// Check if any have finished
		case finished := <-done:
			delete(running, finished)

		// Check if we've received a SIGUSR1
		case _ = <-sigs:
			output, err := json.Marshal(running)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(string(output))
			}
		}
	}
}

func enqueueWriteBackJob(fs *GoCacheFs, relPath string) {
	if fs.WriteBack && !isDeletedFilename(relPath) {
		select {
		case fs.Jobs <- Job{WRITEBACK, toSrcPath(relPath, fs), toDstPath(relPath, fs), fs.MaxWriteBackAttempts}:
			noop()
		default:
			noop()
		}
	}
}

func enqueueDeleteBackJob(fs *GoCacheFs, relPath string) {
	if fs.WriteBack && !isDeletedFilename(relPath) {
		select {
		case fs.Jobs <- Job{DELETEBACK, toSrcPath(relPath, fs), toDstPath(relPath, fs), fs.MaxWriteBackAttempts}:
			noop()
		default:
			noop()
		}
	}
}

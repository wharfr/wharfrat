package internal

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"

	"wharfr.at/wharfrat/lib/version"
)

type Server struct {
}

func (s *Server) Execute(args []string) error {
	log.Printf("Server Args: %#v, Opts: %#v", args, s)

	if err := os.Chmod("/sbin/wr-init", os.ModeSetuid|0755); err != nil {
		return fmt.Errorf("failed to change wr-init permissions: %w", err)
	}

	version.ShowVersion()

	child := make(chan os.Signal, 1)
	signal.Notify(child, unix.SIGCHLD)

	for range child {
		status := unix.WaitStatus(0)
		usage := unix.Rusage{}
		for {
			pid, err := unix.Wait4(-1, &status, unix.WNOHANG, &usage)
			if err != nil && err != unix.ECHILD {
				log.Printf("WAIT4 FAILED: %s", err)
				break
			}
			if pid == 0 || err == unix.ECHILD {
				// no more children to reap
				break
			}
			log.Printf("REAP: %d", pid)
			log.Printf("  STATUS: %d", status)
			log.Printf("  USAGE: %#v", usage)
		}
	}

	return nil
}

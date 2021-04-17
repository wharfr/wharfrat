package internal

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

type Server struct {
}

func (s *Server) Execute(args []string) error {
	log.Printf("Server Args: %#v, Opts: %#v", args, s)

	if err := os.Chmod("/sbin/wr-init", os.ModeSetuid|0755); err != nil {
		return fmt.Errorf("Failed to change wr-init permissions: %w", err)
	}

	child := make(chan os.Signal, 1)
	signal.Notify(child, unix.SIGCHLD)

	for {
		select {
		case <-child:
			status := unix.WaitStatus(0)
			usage := unix.Rusage{}
			pid, err := unix.Wait4(-1, &status, 0, &usage)
			log.Printf("REAP: %d - %s", pid, err)
			log.Printf("  STATUS: %d", status)
			log.Printf("  USAGE: %#v", usage)
		}
	}
}

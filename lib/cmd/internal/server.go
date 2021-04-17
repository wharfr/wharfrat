package internal

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/unix"

	"wharfr.at/wharfrat/lib/version"
)

type Server struct {
}

func (s *Server) Execute(args []string) error {
	log.Printf("Server Args: %#v, Opts: %#v", args, s)

	if err := os.Chmod("/sbin/wr-init", os.ModeSetuid|0755); err != nil {
		return fmt.Errorf("Failed to change wr-init permissions: %w", err)
	}

	version.ShowVersion()

	status := unix.WaitStatus(0)
	usage := unix.Rusage{}
	for {
		pid, err := unix.Wait4(-1, &status, 0, &usage)
		if err != nil {
			log.Printf("WAIT4 FAILED: %s", err)
			continue
		}
		log.Printf("REAP: %d - %s", pid, err)
		log.Printf("  STATUS: %d", status)
		log.Printf("  USAGE: %#v", usage)
	}
}

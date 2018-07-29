package internal

import (
	"fmt"
	"log"
	"os"
)

type Server struct {
}

func (s *Server) Execute(args []string) error {
	log.Printf("Server Args: %#v, Opts: %#v", args, s)

	if err := os.Chmod("/sbin/wr-init", os.ModeSetuid|0755); err != nil {
		return fmt.Errorf("Failed to change wr-init permissions: %s", err)
	}

	select {}
}

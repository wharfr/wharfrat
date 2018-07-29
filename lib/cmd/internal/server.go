package internal

import (
	"log"
)

type Server struct {
}

func (s *Server) Execute(args []string) error {
	log.Printf("Server Args: %#v, Opts: %#v", args, s)

	select {}
}

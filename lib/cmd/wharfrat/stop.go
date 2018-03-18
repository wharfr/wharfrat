package wharfrat

import "log"

type Stop struct {
}

func (s *Stop) Execute(args []string) error {
	log.Fatalf("NOT IMPLEMENTED")
	return nil
}

package wharfrat

import "fmt"

type Stop struct {
}

func (s *Stop) Execute(args []string) error {
	return fmt.Errorf("NOT IMPLEMENTED")
}

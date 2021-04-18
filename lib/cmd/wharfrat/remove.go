package wharfrat

import (
	"fmt"
	"log"
	"strings"

	"wharfr.at/wharfrat/lib/docker"
)

type Remove struct {
	All bool `short:"a" long:"all"`
}

func (s *Remove) Execute(args []string) error {
	log.Printf("REMOVE opts: %#v, args: %s", s, args)

	if s.All {
		if len(args) != 0 {
			return fmt.Errorf("no name allowed with --all")
		}
	} else {
		if len(args) < 1 {
			return fmt.Errorf("at least one container name required")
		}
	}

	names := map[string]bool{}
	for _, name := range args {
		names[name] = true
	}

	client, err := docker.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	containers, err := client.List()
	if err != nil {
		return err
	}

	log.Printf("FOUND: %d", len(containers))

	for _, container := range containers {
		name := strings.TrimPrefix(container.Names[0], "/")

		if !s.All && !names[name] {
			continue
		}

		if err := client.Remove(name, true); err != nil {
			fmt.Printf("Failed to remove %s: %s\n", name, err)
		} else {
			fmt.Printf("%s removed\n", name)
		}
	}

	return nil
}

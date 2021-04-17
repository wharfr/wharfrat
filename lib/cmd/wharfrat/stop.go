package wharfrat

import (
	"fmt"
	"log"
	"strings"

	"wharfr.at/wharfrat/lib/docker"
)

type Stop struct {
	All bool `short:"a" long:"all"`
}

func (s *Stop) Execute(args []string) error {
	log.Printf("STOP opts: %#v, args: %s", s, args)

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

		if s.All || names[name] {
			if err := client.EnsureStopped(name); err != nil {
				fmt.Printf("Failed to stop %s: %s\n", name, err)
			} else {
				fmt.Printf("%s stopped\n", name)
			}
		}
	}

	return nil
}

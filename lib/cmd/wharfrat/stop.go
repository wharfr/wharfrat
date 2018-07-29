package wharfrat

import (
	"fmt"
	"log"
	"strings"

	"git.qur.me/qur/wharf_rat/lib/docker"
)

type Stop struct {
	All bool `short:"a" long:"all"`
}

func (s *Stop) Execute(args []string) error {
	log.Printf("STOP opts: %#v, args: %s", s, args)

	if s.All {
		if len(args) != 0 {
			return fmt.Errorf("No name allowed with --all")
		}
	} else {
		if len(args) < 1 {
			return fmt.Errorf("At least one container name required")
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
		name := container.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}

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

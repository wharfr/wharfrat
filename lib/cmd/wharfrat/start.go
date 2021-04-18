package wharfrat

import (
	"fmt"
	"log"
	"os"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/docker/label"
)

type Start struct {
	All   bool `short:"a" long:"all"`
	Force bool `long:"force" description:"Ignore out of date crate configuration"`
}

func (s *Start) Execute(args []string) error {
	log.Printf("START opts: %#v, args: %s", s, args)

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
		projectFile := container.Labels[label.Project]
		crateName := container.Labels[label.Crate]

		name := strings.TrimPrefix(container.Names[0], "/")

		crate, err := config.OpenCrate(projectFile, crateName, client)
		if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
			return fmt.Errorf("failed to lookup crate: %w", err)
		}

		if s.All || names[name] {
			if crate == nil {
				fmt.Printf("Failed to start %s: crate config missing\n", name)
				continue
			}
			if _, err := client.EnsureRunning(crate, s.Force, false); err != nil {
				fmt.Printf("Failed to start %s: %s\n", name, err)
			} else {
				fmt.Printf("%s started\n", name)
			}
		}
	}

	return nil
}

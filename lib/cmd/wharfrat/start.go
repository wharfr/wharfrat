package wharfrat

import (
	"fmt"
	"log"
	"os"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
)

type Start struct {
	All   bool `short:"a" long:"all"`
	Force bool `long:"force" decription:"Ignore out of date crate configuration"`
}

func (s *Start) Execute(args []string) error {
	log.Printf("START opts: %#v, args: %s", s, args)

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
		projectFile := container.Labels[docker.LabelProject]
		crateName := container.Labels[docker.LabelCrate]

		name := container.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}

		crate, err := config.OpenCrate(projectFile, crateName)
		if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
			return fmt.Errorf("Failed to lookup crate: %s", err)
		}

		if s.All || names[name] {
			if crate == nil {
				fmt.Printf("Failed to start %s: crate config mising\n", name)
				continue
			}
			if _, err := client.EnsureRunning(crate, s.Force); err != nil {
				fmt.Printf("Failed to start %s: %s\n", name, err)
			} else {
				fmt.Printf("%s started\n", name)
			}
		}
	}

	return nil
}

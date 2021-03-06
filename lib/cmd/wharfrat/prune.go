package wharfrat

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/docker/label"
	"wharfr.at/wharfrat/lib/vc"
)

type Prune struct {
	Yes bool `short:"y" long:"yes" description:"Actually remove containers"`
}

func (p *Prune) Execute(args []string) error {
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

	missing := []string{}

	for _, container := range containers {
		projectFile := container.Labels[label.Project]
		crateName := container.Labels[label.Crate]
		branch := container.Labels[label.Branch]

		name := strings.TrimPrefix(container.Names[0], "/")
		project := filepath.Dir(projectFile)
		crate, err := config.OpenCrate(projectFile, crateName, client)
		if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
			return fmt.Errorf("failed to lookup crate: %w", err)
		}

		projectBranch, err := vc.Branch(project)
		if err != nil {
			log.Printf("Failed to get VC branch: %s", err)
		}

		log.Printf("ENTRY %s: %s %s", name, projectBranch, branch)

		if branch != "" && branch != projectBranch {
			crate = nil
			if vc.KnownFile(projectFile, branch) {
				log.Printf("OpenVcCrate: %s %s %s", projectFile, branch, crateName)
				crate, err = config.OpenVcCrate(projectFile, branch, crateName, client)
				if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
					return fmt.Errorf("failed to lookup crate: %w", err)
				}
			}
		}

		if crate == nil {
			missing = append(missing, name)
		}
	}

	log.Printf("MISSING: %v", missing)

	for _, container := range missing {
		if p.Yes {
			if err := client.EnsureRemoved(container); err != nil {
				fmt.Printf("Failed to remove %s: %s\n", container, err)
			} else {
				fmt.Printf("Removed %s\n", container)
			}
		} else {
			fmt.Printf("Would remove %s\n", container)
		}
	}

	if !p.Yes && len(missing) > 0 {
		fmt.Printf("\nRe-run with --yes to remove containers\n")
	}

	return nil
}

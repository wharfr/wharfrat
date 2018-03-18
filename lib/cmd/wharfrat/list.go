package wharfrat

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.qur.me/qur/wharf_rat/lib/config"
	"git.qur.me/qur/wharf_rat/lib/docker"
	"git.qur.me/qur/wharf_rat/lib/vc"
)

type List struct {
}

type triState int

const (
	green triState = iota
	amber
	red
	normal
	dark
)

type strState struct {
	str   string
	state triState
}

type listEntry struct {
	name    string
	project strState
	crate   strState
	image   string
	state   string
	branch  strState
}

func (t triState) fmt() string {
	switch t {
	case green:
		return "\033[32m"
	case amber:
		return "\033[33m"
	case red:
		return "\033[31m"
	case normal:
		return "\033[0m"
	case dark:
		return "\033[30;1m"
	}
	panic("Invalid triState")
}

func dashes(count int) string {
	b := make([]byte, count)
	for i := 0; i < count; i++ {
		b[i] = '-'
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	panic("failed to check path exists")
}

func (l *List) Execute(args []string) error {
	log.Printf("LIST opts: %#v, args: %s", l, args)

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

	entries := []listEntry{}
	maxName, maxProject, maxBranch, maxCrate, maxImage := 14, 14, 14, 5, 5

	for _, container := range containers {
		projectFile := container.Labels["me.qur.wharf-rat.project"]
		crateName := container.Labels["me.qur.wharf-rat.crate"]
		cfg := container.Labels["me.qur.wharf-rat.config"]

		name := container.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}
		project := filepath.Dir(projectFile)
		crate, err := config.OpenCrate(projectFile, crateName)
		if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
			return fmt.Errorf("Failed to lookup crate: %s", err)
		}

		branchState := normal
		branch, err := vc.Branch(project)
		if err != nil {
			log.Printf("Failed to get VC branch: %s", err)
			branch = "<unknown>"
			branchState = dark
		}

		if len(branch) > maxBranch {
			maxBranch = len(branch)
		}

		projectState := green
		if !exists(projectFile) {
			projectState = red
		}

		crateState := green
		if crate == nil {
			crateState = red
		} else if crate.Json() != cfg {
			crateState = amber
		}

		if len(name) > maxName {
			maxName = len(name)
		}

		if len(project) > maxProject {
			maxProject = len(project)
		}

		if len(crateName) > maxCrate {
			maxCrate = len(crateName)
		}

		if len(container.Image) > maxImage {
			maxImage = len(container.Image)
		}

		entries = append(entries, listEntry{
			name:    name,
			project: strState{project, projectState},
			crate:   strState{crateName, crateState},
			image:   container.Image,
			state:   container.State,
			branch:  strState{branch, branchState},
		})
	}

	fmt.Printf("\033[37;1m%-*s\033[0m | ", maxName, "Container Name")
	fmt.Printf("\033[37;1m%-*s\033[0m | ", maxProject, "Project Folder")
	fmt.Printf("\033[37;1m%-*s\033[0m | ", maxBranch, "Project Branch")
	fmt.Printf("\033[37;1m%-*s\033[0m | ", maxCrate, "Crate")
	fmt.Printf("\033[37;1m%-*s\033[0m | ", maxImage, "Image")
	fmt.Printf("\033[37;1m%s\033[0m\n", "Container State")
	fmt.Printf("%s-+-", dashes(maxName))
	fmt.Printf("%s-+-", dashes(maxProject))
	fmt.Printf("%s-+-", dashes(maxBranch))
	fmt.Printf("%s-+-", dashes(maxCrate))
	fmt.Printf("%s-+-", dashes(maxImage))
	fmt.Printf("%s\n", dashes(15))
	for _, entry := range entries {
		fmt.Printf("%-*s", maxName, entry.name)
		fmt.Printf("\033[0m | ")
		fmt.Printf("%s%-*s", entry.project.state.fmt(), maxProject, entry.project.str)
		fmt.Printf("\033[0m | ")
		fmt.Printf("%s%-*s", entry.branch.state.fmt(), maxBranch, entry.branch.str)
		fmt.Printf("\033[0m | ")
		fmt.Printf("%s%-*s", entry.crate.state.fmt(), maxCrate, entry.crate.str)
		fmt.Printf("\033[0m | ")
		fmt.Printf("%-*s", maxImage, entry.image)
		fmt.Printf("\033[0m | ")
		fmt.Printf("%s", entry.state)
		fmt.Printf("\033[0m\n")
	}

	return nil
}
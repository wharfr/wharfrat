package wharfrat

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/vc"
)

type List struct {
	JSON bool `short:"j" long:"json" description:"JSON output instead of table"`
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

type tree map[string]tree

func (t tree) Add(path string) {
	parts := strings.Split(path, string(os.PathSeparator))
	if strings.HasPrefix(path, "/") {
		parts = append([]string{"/"}, parts...)
	}
	t.add(parts)
}

func (t tree) add(path []string) {
	name, rest := path[0], path[1:]
	if _, ok := t[name]; !ok {
		t[name] = tree{}
	}
	if len(rest) != 0 {
		t[name].add(rest)
	}
}

func (t tree) Prefix() string {
	if len(t) != 1 {
		return ""
	}
	for name, sub := range t {
		return filepath.Join(name, sub.Prefix())
	}
	panic("should never reach here")
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
	maxName, maxProject, maxBranch, maxCrate, maxImage := 14, 14, 16, 5, 5

	projects := tree{}

	for _, container := range containers {
		projectFile := container.Labels[docker.LabelProject]
		crateName := container.Labels[docker.LabelCrate]
		cfg := container.Labels[docker.LabelConfig]
		branch := container.Labels[docker.LabelBranch]

		name := container.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}
		project := filepath.Dir(projectFile)
		crate, err := config.OpenCrate(projectFile, crateName)
		if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
			return fmt.Errorf("Failed to lookup crate: %s", err)
		}

		projects.Add(filepath.Dir(project))

		projectBranch, err := vc.Branch(project)
		if err != nil {
			log.Printf("Failed to get VC branch: %s", err)
		}

		projectState := green
		if !exists(projectFile) {
			projectState = red
		}

		branchState := normal
		if branch == "" {
			branch = "<unknown>"
			branchState = dark
		} else if branch != projectBranch {
			if vc.KnownFile(projectFile, branch) {
				log.Printf("OpenVcCrate: %s %s %s", projectFile, branch, crateName)
				crate, err = config.OpenVcCrate(projectFile, branch, crateName)
				if err != nil && !os.IsNotExist(err) && err != config.CrateNotFound {
					return fmt.Errorf("Failed to lookup crate: %s", err)
				}
				projectState = green
			} else {
				projectState = red
				crate = nil
			}
		}

		if len(branch) > maxBranch {
			maxBranch = len(branch)
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

	if l.JSON {
		fmt.Printf("[\n")
		for i, entry := range entries {
			fmt.Printf("  {")
			fmt.Printf(" \"name\": \"%s\",", entry.name)
			fmt.Printf(" \"project\": \"%s\",", entry.project.str)
			fmt.Printf(" \"branch\": \"%s\",", entry.branch.str)
			fmt.Printf(" \"crate\": \"%s\",", entry.crate.str)
			fmt.Printf(" \"image\": \"%s\",", entry.image)
			fmt.Printf(" \"state\": \"%s\"", entry.state)
			fmt.Printf("}")
			if i+1 < len(entries) {
				fmt.Printf(",")
			}
			fmt.Printf("\n")
		}
		fmt.Printf("]\n")
	} else {
		prefix := projects.Prefix()
		log.Printf("PROJECT PREFIX: %s", prefix)
		if len(prefix) > 3 {
			for i, entry := range entries {
				short := strings.Replace(entry.project.str, prefix, "...", 1)
				entries[i].project.str = short
				if len(short) > maxProject {
					maxProject = len(short)
				}
			}
		} else {
			for _, entry := range entries {
				if len(entry.project.str) > maxProject {
					maxProject = len(entry.project.str)
				}
			}
		}

		fmt.Printf("\033[37;1m%-*s\033[0m | ", maxName, "Container Name")
		fmt.Printf("\033[37;1m%-*s\033[0m | ", maxProject, "Project Folder")
		fmt.Printf("\033[37;1m%-*s\033[0m | ", maxBranch, "Container Branch")
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
	}

	return nil
}

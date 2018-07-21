package wharfrat

import (
	"fmt"
	"log"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
)

type Info struct {
	Crate string `short:"c" long:"crate" value-name:"NAME" description:"Name of crate to run"`
}

func (i *Info) Execute(args []string) error {
	log.Printf("INFO: opts: %#v, args: %v", i, args)

	client, err := docker.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	crate, err := config.GetCrate(".", i.Crate, client)
	if err != nil {
		return fmt.Errorf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	project := filepath.Dir(crate.ProjectPath())

	log.Printf("Container: %s", crate.ContainerName())

	cfg := ""
	branch := "n/a"
	addr := "n/a"
	status := "no container"

	container, err := client.GetContainer(crate.ContainerName())
	if err != nil {
		return err
	}

	log.Printf("CONTAINER: %#v", container)

	if container != nil {
		cfg = container.Config.Labels[docker.LabelConfig]
		branch = container.Config.Labels[docker.LabelBranch]

		v4 := container.NetworkSettings.IPAddress
		v6 := container.NetworkSettings.GlobalIPv6Address

		addr = v4
		if v4 != "" && v6 != "" {
			addr = v4 + ", " + v6
		} else if v6 != "" {
			addr = v6
		}

		status = container.State.Status
	}

	fmt.Printf("Project Folder:   %s\n", project)
	fmt.Printf("Crate:            %s\n", crate.Name())
	fmt.Printf("Image:            %s\n", crate.Image)
	fmt.Printf("Container Name:   %s\n", crate.ContainerName())
	fmt.Printf("Container Branch: %s\n", branch)
	fmt.Printf("Container State:  %s\n", status)
	fmt.Printf("Container Stale:  %v\n", cfg != crate.Json())
	fmt.Printf("Container IP:     %s\n", addr)

	return nil
}

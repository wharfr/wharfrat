package wharfrat

import (
	"fmt"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
)

type Run struct {
	Stop    bool   `short:"s" long:"stop" description:"Stop contiainer instead of running command"`
	Crate   string `short:"c" long:"crate" value-name:"NAME" description:"Name of crate to run"`
	Clean   bool   `long:"clean" description:"Rebuild container from Image"`
	User    string `short:"u" long:"user" value-name:"USER[:GROUP]" description:"Override user/group for running command"`
	Workdir string `short:"w" long:"workdir" value-name:"DIR" description:"Override working directory for running command"`
	Force   bool   `long:"force" description:"Ignore out of date crate configuration"`
}

func (opts *Run) stop(args []string) error {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		return fmt.Errorf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		return fmt.Errorf("Failed to create docker client: %s", err)
	}
	defer c.Close()

	if err := c.EnsureStopped(crate.ContainerName()); err != nil {
		return fmt.Errorf("Failed to stop container: %s", err)
	}

	return nil
}

func (opts *Run) client(args []string) (int, error) {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		return 1, fmt.Errorf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		return 1, fmt.Errorf("Failed to create docker client: %s", err)
	}
	defer c.Close()

	if opts.Clean {
		if err := c.EnsureRemoved(crate.ContainerName()); err != nil {
			return 1, fmt.Errorf("Failed to remove container: %s", err)
		}
	}

	container, err := c.EnsureRunning(crate, opts.Force)
	if err != nil {
		return 1, fmt.Errorf("Failed to run container: %s", err)
	}

	if len(args) == 0 {
		args = append(args, "/bin/bash")
	}

	ret, err := c.ExecCmd(container, args, crate, opts.User, opts.Workdir)
	if err != nil {
		return 1, fmt.Errorf("Failed to exec command: %s", err)
	}

	log.Printf("RETCODE: %d", ret)

	return ret, nil
}

func (r *Run) Usage() string {
	return "[run-OPTIONS] [cmd [args...]]"
}

func (r *Run) Execute(args []string) error {
	log.Printf("Args: %#v, Opts: %#v", args, r)

	if r.Stop {
		return r.stop(args)
	}

	ret, err := r.client(args)
	if err != nil {
		return err
	}

	os.Exit(ret)
	return nil
}

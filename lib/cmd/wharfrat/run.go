package wharfrat

import (
	"fmt"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/environ"
	"wharfr.at/wharfrat/lib/venv"
)

type Run struct {
	Stop        bool   `short:"s" long:"stop" description:"Stop container instead of running command"`
	Crate       string `short:"c" long:"crate" value-name:"NAME" description:"Name of crate to run"`
	Clean       bool   `long:"clean" description:"Rebuild container from Image"`
	AutoClean   bool   `long:"auto-clean" description:"Automatically apply --clean, if the container is old"`
	NoAutoClean bool   `long:"no-auto-clean" description:"Disable auto-clean, if enabled in local config"`
	User        string `short:"u" long:"user" value-name:"USER[:GROUP]" description:"Override user/group for running command"`
	Workdir     string `short:"w" long:"workdir" value-name:"DIR" description:"Override working directory for running command"`
	Force       bool   `long:"force" description:"Ignore out of date crate configuration"`
}

func (opts *Run) stop(args []string) error {
	c, err := docker.Connect()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer c.Close()

	crate, err := config.GetCrate(".", opts.Crate, c)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	if err := c.EnsureStopped(crate.ContainerName()); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

func (opts *Run) client(args []string) (int, error) {
	if opts.AutoClean && opts.NoAutoClean {
		return 1, fmt.Errorf("--auto-clean and --no-auto-clean are not compatible")
	}

	c, err := docker.Connect()
	if err != nil {
		return 1, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer c.Close()

	crate, err := config.GetCrate(".", opts.Crate, c)
	if err != nil {
		return 1, fmt.Errorf("config error: %w", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	if environ.InContainer() {
		if len(args) == 0 {
			// nothing to do, interactive session requested, but we are already
			// in container.
			log.Printf("Already in container, nothing to do.")
			return 0, nil
		}
		return environ.Exec(args, crate, opts.User, opts.Workdir)
	}

	if opts.Clean {
		if err := c.EnsureRemoved(crate.ContainerName()); err != nil {
			return 1, fmt.Errorf("failed to remove container: %w", err)
		}
	}

	autoClean := config.Local().AutoClean
	switch {
	case opts.AutoClean:
		autoClean = true
	case opts.NoAutoClean:
		autoClean = false
	}

	container, err := c.EnsureRunning(crate, opts.Force, autoClean)
	if err != nil {
		return 1, fmt.Errorf("failed to run container: %w", err)
	}

	if len(args) == 0 {
		args = append(args, crate.Shell)
	}

	ret, err := c.ExecCmd(container, args, crate, opts.User, opts.Workdir)
	if err != nil {
		return 1, fmt.Errorf("failed to exec command: %w", err)
	}

	log.Printf("RETCODE: %d", ret)

	venv.Update(c, container, crate, opts.User, opts.Workdir, args)

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

package wr

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"git.qur.me/qur/wharf_rat/lib/config"
	"git.qur.me/qur/wharf_rat/lib/docker"

	flags "github.com/jessevdk/go-flags"
)

type Options struct {
	Stop    bool   `short:"s" long:"stop" description:"Stop contiainer instead of running command"`
	Verbose bool   `short:"v" long:"verbose"`
	Crate   string `short:"c" long:"crate" value-name:"NAME" description:"Name of crate to run"`
	Clean   bool   `long:"clean" description:"Rebuild container from Image"`
	User    string `short:"u" long:"user" value-name:"USER[:GROUP]" description:"Override user/group for running command"`
	Workdir string `short:"w" long:"workdir" value-name:"DIR" description:"Override working directory for running command"`
	Debug   bool   `short:"d" long:"debug" description:"Show debug output"`
}

func fatal(msg string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	return 1
}

func stop(opts Options, args []string) int {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		return fatal("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		return fatal("Failed to create docker client: %s", err)
	}
	defer c.Close()

	if err := c.EnsureStopped(crate.ContainerName()); err != nil {
		return fatal("Failed to stop container: %s", err)
	}

	return 0
}

func client(opts Options, args []string) int {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		return fatal("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		return fatal("Failed to create docker client: %s", err)
	}
	defer c.Close()

	if opts.Clean {
		if err := c.EnsureRemoved(crate.ContainerName()); err != nil {
			return fatal("Failed to remove container: %s", err)
		}
	}

	container, err := c.EnsureRunning(crate)
	if err != nil {
		return fatal("Failed to run container: %s", err)
	}

	if len(args) == 0 {
		args = append(args, "/bin/bash")
	}

	ret, err := c.ExecCmd(container, args, crate, opts.User, opts.Workdir)
	if err != nil {
		return fatal("Failed to exec command: %s", err)
	}

	log.Printf("RETCODE: %d", ret)

	return ret
}

func Main() int {
	opts := Options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	if !opts.Debug {
		log.SetOutput(ioutil.Discard)
	}

	log.Printf("Args: %#v, Opts: %#v", args, opts)

	if opts.Stop {
		return stop(opts, args)
	}

	return client(opts, args)
}

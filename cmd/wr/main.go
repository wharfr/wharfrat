package main

import (
	"log"
	"os"

	"git.qur.me/qur/wharf_rat/lib/config"
	"git.qur.me/qur/wharf_rat/lib/docker"

	flags "github.com/jessevdk/go-flags"
)

type Options struct {
	Server  bool   `long:"server" hidden:"true"`
	Stop    bool   `short:"s" long:"stop"`
	Verbose bool   `short:"v" long:"verbose"`
	Crate   string `short:"c" long:"crate"`
}

func server(opts Options, args []string) int {
	select {}
}

func stop(opts Options, args []string) int {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		log.Fatalf("Failed to create docker client: %s", err)
	}
	defer c.Close()

	if err := c.EnsureStopped(crate); err != nil {
		log.Fatalf("Failed to stop container: %s", err)
	}

	return 0
}

func client(opts Options, args []string) int {
	crate, err := config.GetCrate(".", opts.Crate)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	log.Printf("Container: %s", crate.ContainerName())

	c, err := docker.Connect()
	if err != nil {
		log.Fatalf("Failed to create docker client: %s", err)
	}
	defer c.Close()

	container, err := c.EnsureRunning(crate)
	if err != nil {
		log.Fatalf("Failed to run container: %s", err)
	}

	if len(args) == 0 {
		args = append(args, "/bin/bash")
	}

	ret, err := c.ExecCmd(container, args)
	if err != nil {
		log.Fatalf("Failed to exec command: %s", err)
	}

	log.Printf("RETCODE: %d", ret)

	return ret
}

func main() {
	opts := Options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		os.Exit(0)
	} else if err != nil {
		os.Exit(1)
	}
	log.Printf("Args: %#v, Opts: %#v", args, opts)

	if opts.Server {
		os.Exit(server(opts, args))
	}

	if opts.Stop {
		os.Exit(stop(opts, args))
	}

	os.Exit(client(opts, args))
}

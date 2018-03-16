package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

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
}

type ServerOptions struct {
	Server bool     `long:"server"`
	User   string   `short:"u" long:"user" value-name:"USER"`
	Uid    string   `short:"U" long:"uid" value-name:"UID" default:"1000"`
	Group  string   `short:"g" long:"group" value-name:"GROUP"`
	Gid    string   `short:"G" long:"gid" value-name:"GID" default:"1000"`
	Groups []string `short:"e" long:"extra-group" value-name:"GROUP"`
	Name   string   `short:"n" long:"name" value-name:"NAME"`
	Proxy  bool     `short:"p" long:"proxy"`
}

func setup_group(opts ServerOptions) error {
	args := []string{
		"--force",
	}

	if opts.Gid != "0" {
		args = append(args, "--gid", opts.Gid)
	}

	args = append(args, opts.Group)

	log.Printf("groupadd args: %#v", args)

	cmd := exec.Command("/usr/sbin/groupadd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func setup_user(opts ServerOptions) error {
	args := []string{
		"--no-create-home",
	}

	if opts.Uid != "0" {
		args = append(args, "--uid", opts.Uid)
	}

	if opts.Group != "" {
		args = append(args, "--gid", opts.Group, "--no-user-group")
	} else {
		args = append(args, "--user-group")
	}

	if len(opts.Groups) > 0 {
		args = append(args, "--groups", strings.Join(opts.Groups, ","))
	}

	if opts.Name != "" {
		args = append(args, "--comment", opts.Name)
	}

	args = append(args, opts.User)

	log.Printf("useradd args: %#v", args)

	cmd := exec.Command("/usr/sbin/useradd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func server() int {
	opts := ServerOptions{}

	args, err := flags.Parse(&opts)
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}
	log.Printf("Server Args: %#v, Opts: %#v", args, opts)

	if opts.Server && opts.Proxy {
		log.Fatalf("--server and --proxy are not compatible")
	}

	if opts.Group != "" {
		if err := setup_group(opts); err != nil {
			log.Fatalf("Failed to setup group: %s", err)
		}
	}

	if opts.User != "" {
		if err := setup_user(opts); err != nil {
			log.Fatalf("Failed to setup user: %s", err)
		}
	}

	if opts.Server {
		select {}
	}

	if opts.Proxy {
		if len(args) < 2 {
			log.Fatalf("Need at least 2 args for --proxy")
		}
		dir := args[0]
		argv := args[1:]
		log.Printf("PROXY: dir: %s, cmd: %v", dir, argv)

		if err := os.Chdir(dir); err != nil {
			log.Fatalf("Failed to change directory to %s: %s", dir, err)
		}

		cmd, err := exec.LookPath(args[1])
		if err != nil {
			log.Fatalf("Failed to find %s: %s", args[1], err)
		}

		if err := syscall.Exec(cmd, argv, os.Environ()); err != nil {
			log.Fatalf("Failed to exec %s: %s", err)
		}
	}

	return 0
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

	if opts.Clean {
		if err := c.EnsureRemoved(crate); err != nil {
			log.Fatalf("Failed to remove container: %s", err)
		}
	}

	container, err := c.EnsureRunning(crate)
	if err != nil {
		log.Fatalf("Failed to run container: %s", err)
	}

	if len(args) == 0 {
		args = append(args, "/bin/bash")
	}

	ret, err := c.ExecCmd(container, args, crate, opts.User, opts.Workdir)
	if err != nil {
		log.Fatalf("Failed to exec command: %s", err)
	}

	log.Printf("RETCODE: %d", ret)

	return ret
}

func main() {
	if filepath.Base(os.Args[0]) == "wr-init" {
		os.Exit(server())
	}

	opts := Options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		os.Exit(0)
	} else if err != nil {
		os.Exit(1)
	}
	log.Printf("Args: %#v, Opts: %#v", args, opts)

	if opts.Stop {
		os.Exit(stop(opts, args))
	}

	os.Exit(client(opts, args))
}

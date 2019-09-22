package wharfrat

import (
	"fmt"
	"log"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	// "wharfr.at/wharfrat/lib/environ"
	"wharfr.at/wharfrat/lib/venv"
)

type EnvCreate struct {
	Crate string `long:"crate" short:"c" value-name:"NAME" description:"Crate to expose in environment"`
}

type EnvUpdate struct {}

type EnvRebuild struct {}

type EnvInfo struct {}

type Env struct {
	EnvCreate  `command:"create" description:"Create a new environment"`
	EnvUpdate  `command:"update" description:"Update the local wharfrat in the environment"`
	EnvRebuild `command:"rebuild" description:"Force a rebuild of the environment"`
	EnvInfo    `command:"info" description:"Display information about the current environment"`
}

func (ec *EnvCreate) Usage() string {
	return "[create-OPTIONS] <env-path>"
}

func (ec *EnvCreate) Execute(args []string) error {
	log.Printf("Args: %#v, Opts: %#v", args, ec)

	if len(args) < 1 {
		return fmt.Errorf("path to environment required")
	}
	path := args[0]

	c, err := docker.Connect()
	if err != nil {
		return fmt.Errorf("Failed to create docker client: %s", err)
	}
	defer c.Close()

	crate, err := config.GetCrate(".", ec.Crate, c)
	if err != nil {
		return fmt.Errorf("Config error: %s", err)
	}
	log.Printf("Crate: %#v", crate)

	return venv.Create(path, crate, c)
}

func (e *Env) Usage() string {
	return "[env-OPTIONS]"
}

func (eu *EnvUpdate) Execute(args []string) error {
	log.Printf("Args: %#v, Opts: %#v", args, eu)

	return nil
}

func (er *EnvRebuild) Execute(args []string) error {
	log.Printf("Args: %#v, Opts: %#v", args, er)

	return nil
}

func (ei *EnvInfo) Execute(args []string) error {
	venv.DisplayInfo()

	return nil
}
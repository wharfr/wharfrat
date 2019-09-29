package wharfrat

import (
	"fmt"
	"log"

	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/venv"
)

type Env struct {
	EnvCreate  `command:"create" description:"Create a new environment"`
	EnvUpdate  `command:"update" description:"Update the local wharfrat in the environment"`
	EnvInfo    `command:"info" description:"Display information about the current environment"`
}

func (e *Env) Usage() string {
	return "[env-OPTIONS]"
}

type EnvCreate struct {
	Crates []string `long:"crate" short:"c" value-name:"NAME" description:"Crate to expose in environment"`
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

	return venv.Create(path, ec.Crates, c)
}

type EnvUpdate struct {
	Force bool `short:"f" long:"force" description:"Force update, even if commit hash is the same"`
}

func (eu *EnvUpdate) Execute(args []string) error {
	log.Printf("Args: %#v, Opts: %#v", args, eu)

	return venv.UpdateWharfrat(eu.Force)
}

type EnvInfo struct {}

func (ei *EnvInfo) Execute(args []string) error {
	venv.DisplayInfo()

	return nil
}
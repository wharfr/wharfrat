package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Setup struct {
	User   string   `short:"u" long:"user" value-name:"USER"`
	Uid    string   `short:"U" long:"uid" value-name:"UID" default:"1000"`
	Group  string   `short:"g" long:"group" value-name:"GROUP"`
	Gid    string   `short:"G" long:"gid" value-name:"GID" default:"1000"`
	Groups []string `short:"e" long:"extra-group" value-name:"GROUP"`
	Name   string   `short:"n" long:"name" value-name:"NAME"`
	MkHome bool     `short:"h" long:"mkhome"`
}

func (opts *Setup) setup_group() error {
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

func (opts *Setup) setup_user() error {
	args := []string{}

	if opts.MkHome {
		args = append(args, "--create-home")
	} else {
		args = append(args, "--no-create-home")
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

func (s *Setup) Execute(args []string) error {
	log.Printf("Setup Args: %#v, Opts: %#v", args, s)

	if s.Group != "" {
		if err := s.setup_group(); err != nil {
			return fmt.Errorf("Failed to setup group: %s", err)
		}
	}

	if s.User != "" {
		if err := s.setup_user(); err != nil {
			return fmt.Errorf("Failed to setup user: %s", err)
		}
	}

	return nil
}

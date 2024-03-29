package internal

import (
	"bytes"
	"errors"
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
	Create []string `long:"create-group" value-name:"NAME=ID"`
	Name   string   `short:"n" long:"name" value-name:"NAME"`
	MkHome bool     `short:"h" long:"mkhome"`
}

func (s *Setup) create_group(entry string) error {
	parts := strings.SplitN(entry, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("'%s' did not match NAME=ID", entry)
	}

	args := []string{
		"--force", "--gid", parts[1], parts[0],
	}

	log.Printf("groupadd args: %#v", args)

	cmd := exec.Command("/usr/sbin/groupadd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (opts *Setup) setup_group_busybox() error {
	check := exec.Command("/usr/bin/getent", "group", opts.Group)
	switch err := check.Run(); err.(type) {
	case nil:
		return nil
	case *exec.ExitError:
		// group doesn't exist, so run addgroup
	default:
		// some other error ...
		return err
	}

	args := []string{}

	if opts.Gid != "0" {
		exitErr := &exec.ExitError{}
		existing := exec.Command("/usr/bin/getent", "group", opts.Gid)
		switch out, err := existing.Output(); true {
		case err == nil:
			// group already exists with the group id, delete it?
			parts := bytes.Split(out, []byte(":"))
			group := string(bytes.TrimSpace(parts[0]))
			if err := exec.Command("/usr/sbin/delgroup", group).Run(); err != nil {
				return err
			}
		case errors.As(err, &exitErr):
			// gid doesn't exist, so keep going
		default:
			return err
		}
		args = append(args, "-g", opts.Gid)
	}

	args = append(args, opts.Group)

	log.Printf("busybox addgroup args: %#v", args)

	cmd := exec.Command("/usr/sbin/addgroup", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (opts *Setup) setup_group_shadow() error {
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

func (opts *Setup) setup_group() error {
	if path, err := os.Readlink("/usr/sbin/addgroup"); err == nil && strings.HasSuffix(path, "busybox") {
		return opts.setup_group_busybox()
	} else {
		return opts.setup_group_shadow()
	}
}

func (opts *Setup) setup_user_busybox() error {
	args := []string{"-D"}

	if !opts.MkHome {
		args = append(args, "-H")
	}

	if opts.Uid != "0" {
		args = append(args, "-u", opts.Uid)
	}

	if opts.Group != "" {
		args = append(args, "-G", opts.Group)
	}

	// TODO(jp3): We need to add the user to groups listed in opts.Groups

	if opts.Name != "" {
		args = append(args, "-g", opts.Name)
	}

	args = append(args, opts.User)

	log.Printf("busybox adduser args: %#v", args)

	cmd := exec.Command("/usr/sbin/adduser", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (opts *Setup) setup_user_shadow() error {
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

func (opts *Setup) setup_user() error {
	if path, err := os.Readlink("/usr/sbin/adduser"); err == nil && strings.HasSuffix(path, "busybox") {
		return opts.setup_user_busybox()
	} else {
		return opts.setup_user_shadow()
	}
}

func (s *Setup) Execute(args []string) error {
	log.Printf("Setup Args: %#v, Opts: %#v", args, s)

	for _, entry := range s.Create {
		if err := s.create_group(entry); err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}
	}

	if s.Group != "" {
		if err := s.setup_group(); err != nil {
			return fmt.Errorf("failed to setup group: %w", err)
		}
	}

	if s.User != "" {
		if err := s.setup_user(); err != nil {
			return fmt.Errorf("failed to setup user: %w", err)
		}
	}

	return nil
}

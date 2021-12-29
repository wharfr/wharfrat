package internal

import (
	"bytes"
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

func (opts *Setup) create_group_busybox(name, id string) error {
	check := exec.Command("/usr/bin/getent", "group", name)
	switch out, err := check.Output(); err.(type) {
	case nil:
		parts := bytes.Split(bytes.TrimSpace(out), []byte(":"))
		if len(parts) < 3 {
			return fmt.Errorf("failed to parse getent output: %s", out)
		}
		current := string(parts[2])
		if current != id {
			return fmt.Errorf("group %s exists with id %s, wanted %s", name, current, id)
		}
		return nil
	case *exec.ExitError:
		// group doesn't exist, so run addgroup
	default:
		// some other error ...
		return err
	}

	args := []string{
		"-g", id, name,
	}

	log.Printf("busybox addgroup args: %#v", args)

	cmd := exec.Command("/usr/sbin/addgroup", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (s *Setup) create_group_shadow(name, id string) error {
	args := []string{
		"--force", "--gid", id, name,
	}

	log.Printf("groupadd args: %#v", args)

	cmd := exec.Command("/usr/sbin/groupadd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (opts *Setup) create_group(entry string) error {
	parts := strings.SplitN(entry, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("'%s' did not match NAME=ID", entry)
	}
	name, id := parts[0], parts[1]
	if path, err := os.Readlink("/usr/sbin/addgroup"); err == nil && strings.HasSuffix(path, "busybox") {
		return opts.create_group_busybox(name, id)
	} else {
		return opts.create_group_shadow(name, id)
	}
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

func (opts *Setup) add_user_to_group_busybox(group string) error {
	args := []string{
		opts.User, group,
	}

	log.Printf("busybox addgroup args: %#v", args)

	cmd := exec.Command("/usr/sbin/addgroup", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

	if opts.Name != "" {
		args = append(args, "-g", opts.Name)
	}

	args = append(args, opts.User)

	log.Printf("busybox adduser args: %#v", args)

	cmd := exec.Command("/usr/sbin/adduser", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	for _, group := range opts.Groups {
		if err := opts.add_user_to_group_busybox(group); err != nil {
			return err
		}
	}

	return nil
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

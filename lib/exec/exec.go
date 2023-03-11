package exec

import (
	"fmt"
	"log"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/exec/script"
	"wharfr.at/wharfrat/lib/venv"
)

type ExecCfg script.Script

func Parse(path string) (*ExecCfg, error) {
	s, err := script.Parse(path)
	if err != nil {
		return nil, err
	}
	return (*ExecCfg)(s), nil
}

func (e *ExecCfg) getCrate(ls config.LabelSource) (*config.Crate, error) {
	path := e.Project
	base := filepath.Dir(e.Path)
	if path == "" {
		return config.GetCrate(base, e.Crate, ls)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return config.OpenCrate(path, e.Crate, ls)
}

func (e *ExecCfg) Execute(args []string) (int, error) {
	config.Namespace = e.Namespace

	client, err := docker.Connect()
	if err != nil {
		return 0, err
	}
	defer client.Close()
	crate, err := e.getCrate(client)
	if err != nil {
		return 0, err
	}
	log.Printf("CRATE: %#v", crate)
	container, err := client.EnsureRunning(crate, false, e.AutoClean)
	if err != nil {
		return 1, fmt.Errorf("failed to run container: %w", err)
	}
	cmd := e.Command
	if len(cmd) == 0 {
		name := filepath.Base(e.Path)
		cmd = []string{name}
	}
	if e.Meta.IsDefined("args") {
		args = e.Args
	}
	cmd = append(cmd, args...)
	defer venv.Update(client, container, crate, e.User, "", cmd)

	switch e.Version {
	case 1:
		return client.ExecCmd(container, cmd, crate, e.User, "")
	case 2:
		return client.ExecCmd2(container, cmd, crate, e.User, "")
	default:
		return -1, fmt.Errorf("unknown exec version: %d", e.Version)
	}
}

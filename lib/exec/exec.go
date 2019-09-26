package exec

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/burntsushi/toml"
	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/venv"
)

type ExecCfg struct {
	Args      []string `toml:"args"`
	Command   []string `toml:"command"`
	Crate     string   `toml:"crate"`
	Project   string   `toml:"project"`
	User      string   `toml:"user"`
	AutoClean bool     `toml:"auto-clean"`
	path      string
	meta      toml.MetaData
}

func Parse(path string) (*ExecCfg, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var cfg ExecCfg
	md, err := toml.DecodeFile(absPath, &cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("Unknown config keys: %s", md.Undecoded())
	log.Printf("ExecCfg File: %s", absPath)
	log.Printf("ExecCfg: %#v", cfg)
	cfg.path = absPath
	cfg.meta = md
	return &cfg, nil
}

func (e *ExecCfg) getCrate(ls config.LabelSource) (*config.Crate, error) {
	path := e.Project
	base := filepath.Dir(e.path)
	if path == "" {
		return config.GetCrate(base, e.Crate, ls)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return config.OpenCrate(path, e.Crate, ls)
}

func (e *ExecCfg) Execute(args []string) (int, error) {
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
		return 1, fmt.Errorf("Failed to run container: %s", err)
	}
	cmd := e.Command
	if len(cmd) == 0 {
		name := filepath.Base(e.path)
		cmd = []string{name}
	}
	if e.meta.IsDefined("args") {
		args = e.Args
	}
	cmd = append(cmd, args...)
	ret, err := client.ExecCmd(container, cmd, crate, e.User, "")
	if err != nil {
		return -1, err
	}

	venv.Update(client, container, crate, e.User, "", cmd)

	return ret, nil
}

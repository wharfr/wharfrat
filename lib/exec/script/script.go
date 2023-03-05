package script

import (
	"log"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Script struct {
	Args      []string `toml:"args"`
	Command   []string `toml:"command"`
	Crate     string   `toml:"crate"`
	Project   string   `toml:"project"`
	User      string   `toml:"user"`
	AutoClean bool     `toml:"auto-clean"`
	Version   int      `toml:"version"`
	Path      string
	Meta      toml.MetaData
}

func Parse(path string) (*Script, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var cfg Script
	md, err := toml.DecodeFile(absPath, &cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("Unknown config keys: %s", md.Undecoded())
	log.Printf("ExecCfg File: %s", absPath)
	log.Printf("ExecCfg: %#v", cfg)
	cfg.Path = absPath
	cfg.Meta = md
	if cfg.Version == 0 {
		// retroactively make the original version 1
		cfg.Version = 1
	}
	return &cfg, nil
}

package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/burntsushi/toml"
)

type Crate struct {
	Image   string
	Volumes []string
}

type Project struct {
	Crate
	Crates map[string]Crate
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parse(path string) (*Project, error) {
	var project Project
	_, err := toml.DecodeFile(path, &project)
	if err != nil {
		return nil, err
	}
	log.Printf("Project File: %s", path)
	log.Printf("Project: %#v", project)
	return &project, nil
}

func Locate(start string) (*Project, error) {
	fp, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		path := filepath.Join(fp, ".wrproject")
		if exists(path) {
			return parse(path)
		}
		if fp == "/" {
			break
		}
		fp = filepath.Dir(fp)
	}
	return nil, fmt.Errorf(".wrproject not found")
}

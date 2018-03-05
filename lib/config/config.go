package config

import (
	"bytes"
	"io/ioutil"
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

const NotFound = notFound("Not Found")

type notFound string

func (nf notFound) Error() string {
	return string(nf)
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

func find(start, name string) (string, error) {
	fp, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(fp, name)
		if exists(path) {
			return path, nil
		}
		if fp == "/" {
			break
		}
		fp = filepath.Dir(fp)
	}
	return "", NotFound
}

func LocateProject(start string) (*Project, error) {
	path, err := find(start, ".wrproject")
	if err != nil {
		return nil, err
	}
	return parse(path)
}

func LocateCrate(start string) (string, error) {
	path, err := find(start, ".wrcrate")
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(data)), nil
}

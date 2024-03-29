package config

import (
	"log"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Project struct {
	Default string
	Crates  map[string]Crate
	path    string
	meta    toml.MetaData
}

const NotFound = notFound("Not Found")

func parse(path string) (*Project, error) {
	var project Project
	md, err := toml.DecodeFile(path, &project)
	if err != nil {
		return nil, err
	}
	log.Printf("Unknown config keys: %s", md.Undecoded())
	log.Printf("Project File: %s", path)
	log.Printf("Project: %#v", project)
	project.path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	project.meta = md
	return &project, nil
}

func parseStr(data string) (*Project, error) {
	var project Project
	_, err := toml.Decode(data, &project)
	if err != nil {
		return nil, err
	}
	log.Printf("Project: %#v", project)
	return &project, nil
}

func LocateProject(start string) (*Project, error) {
	path, err := find(start, ".wrproject")
	if err != nil {
		return nil, err
	}
	return parse(path)
}

func (p *Project) Path() string {
	return p.path
}
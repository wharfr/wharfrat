package config

import (
	"log"

	"github.com/burntsushi/toml"
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
	project.path = path
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

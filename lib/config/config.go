package config

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/burntsushi/toml"
)

type Crate struct {
	Image       string
	Volumes     []string
	Hostname    string
	Tmpfs       []string
	projectPath string
	name        string
}

type Project struct {
	Default string
	Crates  map[string]Crate
	path    string
}

const NotFound = notFound("Not Found")
const CrateNotFound = notFound("Crate Not Found")

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
	project.path = path
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

func GetCrate(start, name string) (*Crate, error) {
	project, err := LocateProject(start)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse project file: %s", err)
	}
	log.Printf("Project: %#v", project)

	crateName := name
	if crateName == "" {
		crateName, err = LocateCrate(start)
		if err != nil && err != NotFound {
			return nil, fmt.Errorf("Failed to parse crate file: %s", err)
		}
	}

	if crateName == "" {
		crateName = project.Default
	}

	if crateName == "" {
		crateName = "default"
	}

	return openCrate(project, crateName)
}

func openCrate(project *Project, crateName string) (*Crate, error) {
	crate, ok := project.Crates[crateName]
	if !ok {
		return nil, CrateNotFound
	}

	if crate.Hostname == "" {
		crate.Hostname = "dev"
	}

	crate.projectPath = project.path
	crate.name = crateName

	log.Printf("Crate: %s, Image: %s", crateName, crate.Image)

	return &crate, nil
}

func OpenCrate(projectName, crateName string) (*Crate, error) {
	project, err := parse(projectName)
	if err != nil {
		return nil, err
	}
	return openCrate(project, crateName)
}

func (c *Crate) ProjectPath() string {
	return c.projectPath
}

func (c *Crate) Name() string {
	return c.name
}

func (c *Crate) ContainerName() string {
	h := md5.New()
	_, err := h.Write([]byte(c.projectPath))
	if err != nil {
		panic("Failed to write project path: " + err.Error())
	}
	_, err = h.Write([]byte(c.name))
	if err != nil {
		panic("Failed to write project path: " + err.Error())
	}
	hash := hex.EncodeToString(h.Sum(nil))
	return "wr_" + hash
}

func (c *Crate) Json() string {
	b := &strings.Builder{}
	e := json.NewEncoder(b)
	if err := e.Encode(c); err != nil {
		panic("Failed to encode crate to JSON: " + err.Error())
	}
	return b.String()
}

func (c *Crate) Hash() string {
	h := md5.New()
	e := json.NewEncoder(h)
	if err := e.Encode(c); err != nil {
		panic("Failed to encode crate to JSON: " + err.Error())
	}
	return hex.EncodeToString(h.Sum(nil))
}

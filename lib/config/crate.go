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

	"wharfr.at/wharfrat/lib/vc"
)

type Crate struct {
	Image        string
	Volumes      []string
	Hostname     string
	Tmpfs        []string
	Groups       []string
	CapAdd       []string `toml:"cap-add"`
	CapDrop      []string `toml:"cap-drop"`
	SetupPrep    string   `toml:"setup-prep"`
	SetupPre     string   `toml:"setup-pre"`
	SetupPost    string   `toml:"setup-post"`
	Tarballs     map[string]string
	ProjectMount string `toml:"project-mount"`
	WorkingDir   string `toml:"working-dir"`
	MountHome    bool   `toml:"mount-home"`
	Env          map[string]string
	projectPath  string
	name         string
	branch       string
}

const CrateNotFound = notFound("Crate Not Found")

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

	projectDir := filepath.Dir(project.path)
	branch, err := vc.Branch(projectDir)
	if err != nil {
		log.Printf("Failed to get branch name: %s", err)
	}

	return openCrate(project, crateName, branch)
}

func openCrate(project *Project, crateName, branch string) (*Crate, error) {
	crate, ok := project.Crates[crateName]
	if !ok {
		return nil, CrateNotFound
	}

	if !project.meta.IsDefined("crates", crateName, "mount-home") {
		crate.MountHome = true
	}

	if crate.Hostname == "" {
		crate.Hostname = "dev"
	}

	crate.projectPath = project.path
	crate.name = crateName
	crate.branch = branch

	log.Printf("Crate: %s, Image: %s", crateName, crate.Image)

	return &crate, nil
}

func OpenCrate(projectPath, crateName string) (*Crate, error) {
	project, err := parse(projectPath)
	if err != nil {
		return nil, err
	}
	projectDir := filepath.Dir(projectPath)
	branch, err := vc.Branch(projectDir)
	if err != nil {
		log.Printf("Failed to get branch name: %s", err)
	}
	return openCrate(project, crateName, branch)
}

func OpenVcCrate(projectPath, branch, crateName string) (*Crate, error) {
	data, err := vc.BranchedFile(projectPath, branch)
	if err != nil {
		return nil, err
	}
	project, err := parseStr(data)
	if err != nil {
		return nil, err
	}
	project.path = projectPath
	return openCrate(project, crateName, branch)
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
		panic("Failed to write crate name: " + err.Error())
	}
	_, err = h.Write([]byte(c.branch))
	if err != nil {
		panic("Failed to write crate branch: " + err.Error())
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

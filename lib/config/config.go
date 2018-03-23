package config

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.qur.me/qur/wharf_rat/lib/vc"

	"github.com/burntsushi/toml"
	"github.com/docker/docker/api/types"
	"github.com/shibukawa/configdir"
)

type Crate struct {
	Image       string
	Volumes     []string
	Hostname    string
	Tmpfs       []string
	Groups      []string
	projectPath string
	name        string
	branch      string
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

func parseStr(data string) (*Project, error) {
	var project Project
	_, err := toml.Decode(data, &project)
	if err != nil {
		return nil, err
	}
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

func configDir() *configdir.Config {
	configDirs := configdir.New("", "wharf-rat")

	folders := configDirs.QueryFolders(configdir.Global)
	log.Printf("CONFIG FOLDERS: %v", folders)
	return folders[0]
}

func save(filename string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return configDir().WriteFile(filename, content)
}

func load(filename string, data interface{}) error {
	content, err := configDir().ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(content, data)
}

const authFilename = "auth.json"

type Auth map[string]string

func LoadAuth() (Auth, error) {
	auth := Auth{}

	if err := load(authFilename, &auth); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return auth, nil
}

func (a Auth) Set(authConfig *types.AuthConfig) error {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	addr := authConfig.ServerAddress
	a[addr] = base64.URLEncoding.EncodeToString(buf)
	log.Printf("AUTH SET: Addr: %s, Encoded: %s\n", addr, a[addr])
	return nil
}

func (a Auth) Clear(name string) {
	delete(a, name)
}

func (a Auth) Save() error {
	f, err := configDir().Create(authFilename)
	if err != nil {
		return err
	}
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(a)
}

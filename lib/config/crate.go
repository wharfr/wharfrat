package config

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/docker/label"
	"wharfr.at/wharfrat/lib/output"
	"wharfr.at/wharfrat/lib/vc"
)

type LabelSource interface {
	ImageLabels(name string) (map[string]string, error)
}

type Replace map[string]string

func (r Replace) Rewrite(cmd string, w io.Writer, mapping func(string) string) io.Writer {
	out := w
	for match, replace := range r {
		match := os.Expand(match, mapping)
		replace := os.Expand(replace, mapping)
		log.Printf("REPLACE (%s): %s -> %s", cmd, match, replace)
		out = output.NewRewriter(out, []byte(match), []byte(replace))
	}
	return out
}

type Crate struct {
	CapAdd       []string           `toml:"cap-add"`
	CapDrop      []string           `toml:"cap-drop"`
	CopyGroups   []string           `toml:"copy-groups"`
	CmdReplace   map[string]Replace `toml:"cmd-replace"`
	Env          map[string]string  `toml:"env"`
	EnvBlacklist []string           `toml:"env-blacklist"`
	EnvWhitelist []string           `toml:"env-whitelist"`
	ExportBin    []string           `toml:"export-bin"`
	Groups       []string           `toml:"groups"`
	Hostname     string             `toml:"hostname"`
	Image        string             `toml:"image"`
	ImageCmd     string             `toml:"image-cmd"`
	MountHome    bool               `toml:"mount-home"`
	Network      string             `toml:"network"`
	PathAppend   []string           `toml:"path-append"`
	PathPrepend  []string           `toml:"path-prepend"`
	Ports        []string           `toml:"ports"`
	ProjectMount string             `toml:"project-mount"`
	SetupPost    string             `toml:"setup-post"`
	SetupPre     string             `toml:"setup-pre"`
	SetupPrep    string             `toml:"setup-prep"`
	Shell        string             `toml:"shell"`
	Tarballs     map[string]string  `toml:"tarballs"`
	Tmpfs        []string           `toml:"tmpfs"`
	Volumes      []string           `toml:"volumes"`
	WorkingDir   string             `toml:"working-dir"`
	project      *Project           `toml:"-"`
	name         string             `toml:"-"`
	branch       string             `toml:"-"`
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

func GetCrate(start, name string, ls LabelSource) (*Crate, error) {
	project, err := LocateProject(start)
	if err != nil {
		return nil, fmt.Errorf("failed to parse project file: %w", err)
	}
	log.Printf("Project: %#v", project)

	crateName := name
	if crateName == "" {
		crateName, err = LocateCrate(start)
		if err != nil && err != NotFound {
			return nil, fmt.Errorf("failed to parse crate file: %w", err)
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

	return openCrate(project, crateName, branch, ls)
}

func runImageCmd(command string, projectDir string) (string, error) {
	command = strings.TrimSpace(command)
	shell := []string{"sh"}
	if strings.HasPrefix(command, "#!") {
		hashBang := strings.Split(command, "\n")[0]
		shell = strings.Split(strings.TrimSpace(hashBang[2:]), " ")
	}
	buf := &bytes.Buffer{}
	cmd := exec.Command(shell[0], shell[1:]...)
	cmd.Stdin = strings.NewReader(command)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"WHARFRAT_PROJECT_DIR="+projectDir,
	)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func openCrate(project *Project, crateName, branch string, ls LabelSource) (*Crate, error) {
	crate, ok := project.Crates[crateName]
	if !ok {
		return nil, CrateNotFound
	}

	if crate.ImageCmd != "" {
		image, err := runImageCmd(crate.ImageCmd, filepath.Dir(project.path))
		if err != nil {
			return nil, fmt.Errorf("image-cmd failed: %s", err)
		}
		if image != "" {
			crate.Image = image
		}
	}

	if crate.Image == "" {
		return nil, fmt.Errorf("image is a required parameter")
	}

	crate.project = project
	crate.name = crateName
	crate.branch = branch

	if err := crate.SetDefaults(ls); err != nil {
		return nil, err
	}

	log.Printf("Crate: %s, Image: %s", crateName, crate.Image)

	return &crate, nil
}

func OpenCrate(projectPath, crateName string, ls LabelSource) (*Crate, error) {
	project, err := parse(projectPath)
	if err != nil {
		return nil, err
	}

	if crateName == "" {
		crateName = project.Default
	}

	if crateName == "" {
		crateName = "default"
	}

	projectDir := filepath.Dir(projectPath)
	branch, err := vc.Branch(projectDir)
	if err != nil {
		log.Printf("Failed to get branch name: %s", err)
	}
	return openCrate(project, crateName, branch, ls)
}

func OpenVcCrate(projectPath, branch, crateName string, ls LabelSource) (*Crate, error) {
	data, err := vc.BranchedFile(projectPath, branch)
	if err != nil {
		return nil, err
	}
	project, err := parseStr(data)
	if err != nil {
		return nil, err
	}
	project.path = projectPath
	return openCrate(project, crateName, branch, ls)
}

func (c *Crate) SetDefaults(ls LabelSource) error {
	if !c.project.meta.IsDefined("crates", c.name, "mount-home") {
		c.MountHome = true
	}

	if !c.project.meta.IsDefined("crates", c.name, "hostname") {
		c.Hostname = ""
	}

	if !c.project.meta.IsDefined("crates", c.name, "shell") {
		c.Shell = ""
	}

	labels, err := ls.ImageLabels(c.Image)
	if err != nil {
		return err
	}

	if c.Hostname == "" {
		c.Hostname = "dev"
	}

	if c.Shell == "" {
		// Initially we look at the image labels to see if there is a shell
		// specified with the image
		c.Shell = labels[label.Shell]
	}

	if c.Shell == "" {
		// First default is the user's current shell
		c.Shell = os.Getenv("SHELL")
	}
	if c.Shell == "" {
		// Final fallback is /bin/sh
		c.Shell = "/bin/sh"
	}

	return nil
}

func (c *Crate) ProjectPath() string {
	return c.project.path
}

func (c *Crate) Name() string {
	return c.name
}

func (c *Crate) ContainerName() string {
	h := md5.New()
	_, err := h.Write([]byte(c.project.path))
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
	usr, err := user.Current()
	if err != nil {
		panic("Failed to get user information: " + err.Error())
	}
	_, err = h.Write([]byte(usr.Username))
	if err != nil {
		panic("Failed to write username: " + err.Error())
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

func (c *Crate) Getenv(name string) string {
	switch name {
	case "WHARFRAT_NAME":
		return c.ContainerName()
	case "WHARFRAT_CRATE":
		return c.Name()
	case "WHARFRAT_PROJECT":
		return c.ProjectPath()
	case "WHARFRAT_PROJECT_DIR":
		return filepath.Dir(c.ProjectPath())
	default:
		return os.Getenv(name)
	}
}

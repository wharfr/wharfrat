package venv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
)

type binary struct {
	Command []string `json:"command"`
	Paths   []string `json:"paths"`
}

type state struct {
	Project  string        `toml:"project"`
	Crate    string        `toml:"crate"`
	Binaries []binary      `json:"binaries"`
	EnvPath  string        `json:"envpath"`
}

func copySelf(dest string) error {
	in, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if err := os.Chmod(dest, 0755); err != nil {
		os.Remove(dest)
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dest)
		return err
	}

	return nil
}

func ensure(path string) error {
	binPath := filepath.Join(path, "bin")
	if _, err := os.Stat(binPath); err == nil {
		// bin path exists, so assume entire environment has been setup
		return nil
	}
	if err := os.MkdirAll(binPath, 0755); err != nil {
		return err
	}
	wr := filepath.Join(binPath, "wharfrat")
	if err := copySelf(wr); err != nil {
		return err
	}
	if err := os.Symlink(wr, filepath.Join(binPath, "wr")); err != nil {
		return err
	}
	if err := os.Symlink(wr, filepath.Join(binPath, "wr-exec")); err != nil {
		return err
	}
	return nil
}

func getBinaries(c *docker.Connection, id string, crate *config.Crate, user string, patterns []string) ([]string, error) {
	search := []string{"/sbin/wr-init", "search", "-x"}
	search = append(search, patterns...)
	stdout, stderr, err := c.GetOutput(id, search, crate, user)
	log.Printf("OUTPUT: %s %s %s", stdout, stderr, err)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(stdout, []byte("\n"))
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			paths = append(paths, string(line))
		}
	}
	return paths, nil
}

func (s *state) getDelta(paths []string) []string {
	old := map[string]bool{}
	for _, bin := range s.Binaries {
		for _, path := range bin.Paths {
			old[path] = true
		}
	}
	delta := make([]string, 0, len(paths))
	for _, path := range paths {
		if !old[path] {
			delta = append(delta, path)
		}
	}
	log.Printf("DELTA: %s -> %s", paths, delta)
	return delta
}

func Create(relPath string, crate *config.Crate, c *docker.Connection) error {
	path, err := filepath.Abs(relPath)
	if err != nil {
		return fmt.Errorf("failed to convert %s to absolute path: %s", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat %s: %s", err)
	}
	if err := ensure(path); err != nil {
		return fmt.Errorf("failed to setup environment: %s", err)
	}
	s := state{
		Project: crate.ProjectPath(),
		Crate:   crate.Name(),
		EnvPath: path,
	}
	container, err := c.GetContainer(crate.ContainerName())
	if err != nil {
		return fmt.Errorf("Failed to get docker container: %s", err)
	}
	if container != nil {
		if err := s.Update(c, container.ID, crate, "", nil); err != nil {
			return fmt.Errorf("failed to update exported binaries: %s", err)
		}
	}
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save state: %s", err)
	}
	return nil
}

func loadState() (*state, error) {
	envPath := os.Getenv("WHARFRAT_ENV")
	if envPath == "" {
		// no environment enabled
		return nil, nil
	}
	s := state{}
	statePath := filepath.Join(envPath, ".state.json")
	f, err := os.Open(statePath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	if s.EnvPath != envPath {
		return nil, fmt.Errorf("environment may have been moved?")
	}
	return &s, nil
}

func (s *state) MatchesCrate(crate *config.Crate) bool {
	return crate.Name() == s.Crate && crate.ProjectPath() == s.Project
}

func (s *state) createBinary(path string, cmd []string) error {
	_, name := filepath.Split(path)
	refPath := filepath.Join(s.EnvPath, "bin", name)
	f, err := os.Create(refPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.Chmod(refPath, 0755); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("#!%s/bin/wr-exec\n\n", s.EnvPath)); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("project = \"%s\"\n", s.Project)); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("crate = \"%s\"\n", s.Crate)); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("command = [\"%s\"]\n", path)); err != nil {
		os.Remove(refPath)
		return err
	}
	return nil
}

func (s *state) exportBinaries(cmd []string, paths[]string) error {
	log.Printf("EXPORT: %s %s", cmd, paths)
	for _, path := range paths {
		if err := s.createBinary(path, cmd); err != nil {
			return err
		}
	}
	s.Binaries = append(s.Binaries, binary{
		Command: cmd,
		Paths:   paths,
	})
	return nil
}

func (s *state) Save() error {
	statePath := filepath.Join(s.EnvPath, ".state.json")
	f, err := os.Create(statePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(&s); err != nil {
		return err
	}
	return nil
}

func (s *state) Update(c *docker.Connection, id string, crate *config.Crate, user string, cmd []string) error {
	paths, err := getBinaries(c, id, crate, user, crate.ExportBin)
	log.Printf("OUTPUT: %s %s %s", cmd, paths, err)
	if err != nil {
		log.Printf("ERROR: Failed to update exported binaries: %s", err)
		return err
	}
	delta := s.getDelta(paths)
	if len(delta) == 0 {
		// Nothing changed
		return nil
	}
	if err := s.exportBinaries(cmd, delta); err != nil {
		log.Printf("ERROR: Failed to export binaries: %s", err)
		return err
	}
	return nil
}

func Update(c *docker.Connection, id string, crate *config.Crate, user string, cmd []string) {
	if len(crate.ExportBin) == 0 {
		// no export paths configured, so do nothing
		return
	}
	state, err := loadState()
	if err != nil {
		log.Printf("ERROR: Failed to create environment: %s", err)
		return
	}
	if state == nil || !state.MatchesCrate(crate) {
		// environment is either not enabled, or for another project/crate
		return
	}
	if err := state.Update(c, id, crate, user, cmd); err != nil {
		log.Printf("ERROR: Failed to update exported binaries: %s", err)
		return
	}
	if err := state.Save(); err != nil {
		log.Printf("ERROR: Failed to save state: %s", err)
		return
	}
}

func (s *state) Delete() {
	if err := os.RemoveAll(s.EnvPath); err != nil {
		log.Printf("ERROR: Failed to delete environment: %s", err)
		return
	}
}

func Rebuild(c *docker.Connection, id string, crate *config.Crate) {
	state, err := loadState()
	if err != nil {
		log.Printf("ERROR: Failed to load state: %s", err)
		return
	}
	if state == nil || !state.MatchesCrate(crate) {
		// environment is either not enabled, or for another project/crate
		return
	}
	state.Delete()
	Create(state.EnvPath, crate, c)
}

func DisplayInfo() {
	state, err := loadState()
	if err != nil {
		log.Printf("ERROR: Failed to load state: %s", err)
		fmt.Println("Failed to load environment state")
		return
	}
	if state == nil {
		fmt.Println("No Environment")
		return
	}
	fmt.Printf("Path: %s\n", state.EnvPath)
	fmt.Printf("Project: %s\n", state.Project)
	fmt.Printf("Crate: %s\n", state.Crate)
}

func init() {
	docker.AfterCreate(Rebuild)
}
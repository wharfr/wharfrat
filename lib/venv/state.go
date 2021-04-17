package venv

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
)

type binary struct {
	Command []string `json:"command"`
	Paths   []string `json:"paths"`
	User    string   `json:"user"`
	Workdir string   `json:"workdir"`
}

type state struct {
	Project  string              `json:"project"`
	Crates   []string            `json:"crates"`
	Binaries map[string][]binary `json:"binaries"`
	EnvPath  string              `json:"envpath"`
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

func newState(path, project string, crates []string, c *docker.Connection) (*state, error) {
	s := &state{
		Project:  project,
		Crates:   crates,
		EnvPath:  path,
		Binaries: map[string][]binary{},
	}
	for _, name := range crates {
		crate, err := config.GetCrate(".", name, c)
		if err == config.CrateNotFound {
			return nil, fmt.Errorf("unknown crate: %s", name)
		} else if err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
		log.Printf("Crate: %#v", crate)
		id, err := c.EnsureRunning(crate, false, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get running container: %w", err)
		}
		if err := s.Update(c, id, crate, "", "", nil); err != nil {
			return nil, fmt.Errorf("failed to update exported binaries: %w", err)
		}
	}
	return s, nil
}

func (s *state) getDelta(crate string, paths []string) []string {
	old := map[string]bool{}
	for _, bin := range s.Binaries[crate] {
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

func (s *state) MatchesCrate(crate *config.Crate) bool {
	if crate.ProjectPath() != s.Project {
		return false
	}
	for _, name := range s.Crates {
		if crate.Name() == name {
			return true
		}
	}
	return false
}

func (s *state) createBinary(crate, path string) error {
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
	if _, err := f.WriteString(fmt.Sprintf("crate = \"%s\"\n", crate)); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("command = [\"%s\"]\n", path)); err != nil {
		os.Remove(refPath)
		return err
	}
	if _, err := f.WriteString("auto-clean = true\n"); err != nil {
		os.Remove(refPath)
		return err
	}
	return nil
}

func (s *state) exportBinaries(crate string, cmd []string, user, workdir string, paths []string) error {
	log.Printf("EXPORT: %s %s", cmd, paths)
	for _, path := range paths {
		if err := s.createBinary(crate, path); err != nil {
			return err
		}
	}
	s.Binaries[crate] = append(s.Binaries[crate], binary{
		Command: cmd,
		Paths:   paths,
		User:    user,
		Workdir: workdir,
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

func (s *state) restoreItem(c *docker.Connection, id string, crate *config.Crate, bin binary) error {
	if len(bin.Command) == 0 {
		// no command - no nothing to run
		return nil
	}
	delta := s.getDelta(crate.Name(), bin.Paths)
	if len(delta) == 0 {
		return nil
	}
	log.Printf("RESTORE: %s %s", bin.Command, delta)
	ret, err := c.ExecCmd(id, bin.Command, crate, bin.User, bin.Workdir)
	if err != nil {
		return fmt.Errorf("failed to exec command (%s): %w", bin.Command, err)
	}
	if ret != 0 {
		return fmt.Errorf("running command (%s) failed", bin.Command)
	}
	if err := s.Update(c, id, crate, bin.User, bin.Workdir, bin.Command); err != nil {
		return fmt.Errorf("failed to update exported binaries: %w", err)
	}
	return nil
}

func (s *state) findExternalWharfrat() (string, error) {
	binPath := filepath.Join(s.EnvPath, "bin")
	return lookPath("wharfrat", binPath)
}

func (s *state) Update(c *docker.Connection, id string, crate *config.Crate, user, workdir string, cmd []string) error {
	paths, err := getBinaries(c, id, crate, user, crate.ExportBin)
	log.Printf("OUTPUT: %s %s %s", cmd, paths, err)
	if err != nil {
		log.Printf("ERROR: Failed to update exported binaries: %s", err)
		return err
	}
	delta := s.getDelta(crate.Name(), paths)
	if len(delta) == 0 {
		// Nothing changed
		return nil
	}
	if err := s.exportBinaries(crate.Name(), cmd, user, workdir, delta); err != nil {
		log.Printf("ERROR: Failed to export binaries: %s", err)
		return err
	}
	return nil
}

func (s *state) Delete() {
	if err := os.RemoveAll(s.EnvPath); err != nil {
		log.Printf("ERROR: Failed to delete environment: %s", err)
		return
	}
}

func (s *state) Rebuild(c *docker.Connection, id string, crate *config.Crate) error {
	name := crate.Name()
	binaries, ok := s.Binaries[name]
	if !ok {
		// This crate doesn't have any exported binaries
		return nil
	}
	s.Binaries[name] = nil
	if err := s.Update(c, id, crate, "", "", nil); err != nil {
		return fmt.Errorf("failed to update exported binaries: %s", err)
	}
	for _, bin := range binaries {
		if err := s.restoreItem(c, id, crate, bin); err != nil {
			return err
		}
	}
	return nil
}

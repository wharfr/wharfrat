package venv

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/version"
)

func Create(relPath string, crates []string, c *docker.Connection) error {
	proj, err := config.LocateProject(".")
	if err != nil {
		return fmt.Errorf("failed to find project: %s", err)
	}
	if len(crates) == 0 {
		crates = getCrateNames(proj)
	}
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
	s, err := newState(path, proj.Path(), crates, c)
	if err != nil {
		os.RemoveAll(path)
		return err
	}
	if err := s.Save(); err != nil {
		os.RemoveAll(path)
		return fmt.Errorf("failed to save state: %s", err)
	}
	return nil
}

func Update(c *docker.Connection, id string, crate *config.Crate, user, workdir string, cmd []string) {
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
	if err := state.Update(c, id, crate, user, workdir, cmd); err != nil {
		log.Printf("ERROR: Failed to update exported binaries: %s", err)
		return
	}
	if err := state.Save(); err != nil {
		log.Printf("ERROR: Failed to save state: %s", err)
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
	if err := state.Rebuild(c, id, crate); err != nil {
		log.Printf("ERROR: Failed to rebuild environment: %s", err)
	}
	if err := state.Save(); err != nil {
		log.Printf("ERROR: Failed to save state: %s", err)
		return
	}
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
	fmt.Printf("Crates: %s\n", strings.Join(state.Crates, ", "))
}

func UpdateWharfrat(force bool) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %s", err)
	}
	if state == nil {
		return fmt.Errorf("environment not activated")
	}
	externalWharfrat, err := state.findExternalWharfrat()
	if err != nil {
		return fmt.Errorf("failed to find wharfrat command: %s", err)
	}
	if externalWharfrat == "" {
		// there is no external wharfrat
		return nil
	}
	log.Printf("External Wharfrat: %s", externalWharfrat)
	externalCommit, err := getExternalCommit(externalWharfrat)
	if err != nil {
		return fmt.Errorf("failed to get version of external wharfrat: %s", err)
	}
	if externalCommit == version.Commit() && !force {
		// already up to date
		log.Printf("Same commit hash: %s", version.Commit())
		return nil
	}
	log.Printf("UPDATING: %s -> %s", version.Commit(), externalCommit)
	wrPath := filepath.Join(state.EnvPath, "bin", "wharfrat")
	if err := os.Remove(wrPath); err != nil {
		return fmt.Errorf("failed to remove old wharfrat: %s", err)
	}
	if err := copyFile(wrPath, externalWharfrat); err != nil {
		return fmt.Errorf("failed to copy new wharfrat: %s", err)
	}
	return nil
}

func init() {
	docker.AfterCreate(Rebuild)
}

package venv

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/version"
)

func copySelf(dest string) error {
	return copyFile(dest, "/proc/self/exe")
}

func copyFile(dest, source string) error {
	in, err := os.Open(source)
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

func getCrateNames(project *config.Project) []string {
	crates := make([]string, 0, len(project.Crates))
	for name := range project.Crates {
		crates = append(crates, name)
	}
	return crates
}

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
	s := state{
		Project:  proj.Path(),
		Crates:   crates,
		EnvPath:  path,
		Binaries: map[string][]binary{},
	}
	for _, name := range crates {
		crate, err := config.GetCrate(".", name, c)
		if err == config.CrateNotFound {
			os.RemoveAll(path)
			return fmt.Errorf("Unknown crate: %s", crate)
		} else if err != nil {
			os.RemoveAll(path)
			return fmt.Errorf("Config error: %s", err)
		}
		log.Printf("Crate: %#v", crate)
		container, err := c.GetContainer(crate.ContainerName())
		if err != nil {
			os.RemoveAll(path)
			return fmt.Errorf("Failed to get docker container: %s", err)
		}
		if container != nil {
			if err := s.Update(c, container.ID, crate, "", "", nil); err != nil {
				os.RemoveAll(path)
				return fmt.Errorf("failed to update exported binaries: %s", err)
			}
		}
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

func getExternalCommit(tool string) (string, error) {
	cmd := exec.Command(tool, "version", "--commit")
	out, err := cmd.Output()
	return string(bytes.TrimSpace(out)), err
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

package venv

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/self"
)

func copySelf(dest string) error {
	path, err := self.Native()
	if err != nil {
		return err
	}
	return copyFile(dest, path)
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

func getExternalCommit(tool string) (string, error) {
	cmd := exec.Command(tool, "version", "--commit")
	out, err := cmd.Output()
	return string(bytes.TrimSpace(out)), err
}

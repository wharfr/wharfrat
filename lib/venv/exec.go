package venv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// lookPath is exec.LookPath from the standard library, except that it ignores
// the given directory when searching PATH
func lookPath(file, ignore string) (string, error) {
	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &exec.Error{Name: file, Err: err}
	}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == ignore {
			continue
		}
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

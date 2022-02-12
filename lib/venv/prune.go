package venv

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/exec/script"
)

func Prune(remove bool) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %s", err)
	}
	if state == nil {
		return fmt.Errorf("environment not activated")
	}

	client, err := docker.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	cfgs, err := findExecScripts(state)
	if err != nil {
		return err
	}

	for crate, scripts := range cfgs {
		if err := pruneCrate(client, state, crate, scripts, remove); err != nil {
			return err
		}
	}

	return nil
}

func findExecScripts(state *state) (map[string]map[string]string, error) {
	binDir := filepath.Join(state.EnvPath, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		paths = append(paths, filepath.Join(binDir, entry.Name()))
	}
	log.Printf("ENTRIES: %s", paths)
	scripts := map[string]map[string]string{}
	for _, path := range paths {
		if b, err := isExecScript(path); err != nil {
			return nil, err
		} else if !b {
			log.Printf("Ignore non-script: %s", path)
			continue
		}
		c, err := script.Parse(path)
		if err != nil {
			return nil, err
		}
		if scripts[c.Crate] == nil {
			scripts[c.Crate] = make(map[string]string)
		}
		scripts[c.Crate][path] = c.Command[0]
	}
	log.Printf("SCRIPTS: %s", scripts)
	return scripts, nil
}

func isExecScript(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	firstLine, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return false, err
	}
	log.Printf("Check first line (%s): %s", path, firstLine)
	if strings.HasPrefix(firstLine, "#!") && strings.HasSuffix(firstLine, "/wr-exec\n") {
		return true, nil
	}
	return false, nil
}

func pruneCrate(c *docker.Connection, state *state, crateName string, scripts map[string]string, remove bool) error {
	crate, err := config.OpenCrate(state.Project, crateName, c)
	if err != nil {
		return err
	}
	container, err := c.EnsureRunning(crate, false, false)
	if err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}
	paths, err := getBinaries(c, container, crate, "", crate.ExportBin)
	if err != nil {
		return err
	}
	log.Printf("PATHS: %s", paths)
	targets := map[string]bool{}
	for _, target := range paths {
		targets[target] = true
	}
	missing := []string{}
	for script, target := range scripts {
		if targets[target] {
			continue
		}
		log.Printf("MISSING: %s", script)
		missing = append(missing, script)
	}
	if len(missing) == 0 {
		return nil
	}

	fmt.Printf("Scripts with missing commands:\n")
	for _, script := range missing {
		fmt.Printf("  %s\n", script)
	}

	if !remove {
		fmt.Printf("\nre-run with -r/--remove to remove\n")
		return nil
	}

	for _, script := range missing {
		if err := os.Remove(script); err != nil {
			log.Printf("ERROR: Failed to remove %s: %s", script, err)
			fmt.Printf("Failed to remove %s", script)
		}
	}

	return nil
}

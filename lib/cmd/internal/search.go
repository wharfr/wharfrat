package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Search struct {
	All        bool `short:"a"`
	Executable bool `short:"x"`
	File       bool `short:"f"`
	Directory  bool `short:"d"`
}

func (s *Search) wanted(path string) bool {
	if s.All {
		return true
	}
	stat, err := os.Stat(path)
	if err != nil {
		log.Printf("ERROR: Failed to check path %s: %s", path, err)
		return false
	}
	if s.Directory && stat.IsDir() {
		return true
	}
	if !stat.Mode().IsRegular() {
		// must be a regular file past this point
		return false
	}
	if s.File {
		return true
	}
	if s.Executable && stat.Mode().Perm() & 0111 != 0 {
		return true
	}
	return false
}

func (s *Search) Execute(args []string) error {
	for _, pattern := range args {
		pattern = os.ExpandEnv(pattern)
		log.Printf("PATTERN: %s", pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			if s.wanted(path) {
				fmt.Println(path)
			}
		}
	}
	return nil
}
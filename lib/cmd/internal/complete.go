package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Complete struct {
	Line     string `short:"l" long:"line"`
	Current  int    `short:"c" long:"current" default:"-1"`
	Point    int    `short:"p" long:"point" default:"-1"`
	cmdStart int
}

func (c *Complete) isExecutable(info os.FileInfo) bool {
	sys := info.Sys()
	log.Printf("Sys: %T %v", sys, sys)
	return info.Mode().Perm()&0111 != 0
}

func (c *Complete) markDirs(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			log.Printf("Failed to stat %s: %s", path, err)
			continue
		}
		if info.IsDir() {
			filtered = append(filtered, path+"/")
		} else {
			filtered = append(filtered, path)
		}
	}
	log.Printf("Filtered: %v", filtered)
	return filtered
}

func (c *Complete) matchExec(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			log.Printf("Failed to stat %s: %s", path, err)
			continue
		}
		if info.IsDir() {
			filtered = append(filtered, path+"/")
		} else if c.isExecutable(info) {
			filtered = append(filtered, path)
		}
	}
	log.Printf("Filtered: %v", filtered)
	return filtered
}

func (c *Complete) completePath(path string) []string {
	matches, err := filepath.Glob(path + "*")
	log.Printf("matches: %v, err: %s", matches, err)
	if err == nil {
		return matches
	}
	return nil
}

func (c *Complete) completeWord(word string) error {
	matches := []string{}

	if strings.HasPrefix(word, "/") || strings.HasPrefix(word, "./") {
		matches = c.completePath(word)
	}

	if c.Current == c.cmdStart {
		matches = c.matchExec(matches)
	} else {
		matches = c.markDirs(matches)
	}

	if len(matches) == 1 && matches[0] != word {
		return c.completeWord(matches[0])
	}

	for _, match := range matches {
		fmt.Printf("%s\n", match)
	}

	return nil
}

func (c *Complete) findCmdStart(args []string) {
	c.cmdStart = -1
	if len(args) < 2 {
		return
	}
	// TODO(jp3): This is quite possibly wrong, but how do we recognise options
	// that take arguments? We also need to know if the command takes a list of
	// arguments.
	c.cmdStart = 1
}

func (c *Complete) Execute(args []string) error {
	log.Printf("words: %v, current: %d, line: %s, point: %d", args, c.Current, c.Line, c.Point)

	if c.Current < 0 || c.Current >= len(args) {
		log.Printf("current does not index into args ...")
		return nil
	}

	c.findCmdStart(args)

	word := args[c.Current]

	return c.completeWord(word)
}

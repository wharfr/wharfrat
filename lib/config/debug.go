package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

var Debug = false

type coloured struct {
	c *color.Color
}

func (c *coloured) Write(b []byte) (int, error) {
	if c.c == nil {
		return os.Stderr.Write(b)
	}
	if _, err := c.c.Fprint(color.Error, string(b)); err != nil {
		return 0, err
	}
	return len(b), nil
}

func SetupLogging(debug bool) {
	Debug = debug
	name := filepath.Base(os.Args[0])
	log.SetPrefix(fmt.Sprintf("%s: ", strings.ToUpper(name)))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var c *color.Color
	switch strings.ToLower(name) {
	case "wr", "wharfrat":
		c = color.New(color.FgHiYellow)
	case "wr-exec":
		c = color.New(color.FgHiBlue)
	case "wr-init":
		c = color.New(color.FgHiBlack)
	}
	// force color on, even if stdout isn't a terminal
	c.EnableColor()
	if debug {
		log.SetOutput(&coloured{c: c})
	}
}

func init() {
	// Disable logging until enabled by SetupLogging
	log.SetOutput(io.Discard)
}

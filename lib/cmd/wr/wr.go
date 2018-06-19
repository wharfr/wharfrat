package wr

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/cmd/wharfrat"
	"wharfr.at/wharfrat/lib/version"

	flags "github.com/jessevdk/go-flags"
)

type options struct {
	wharfrat.Run
	Debug   bool `short:"d" long:"debug" description:"Show debug output"`
	Version bool `long:"version" description:"Show version of tool"`
}

func fatal(msg string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	return 1
}

func Main() int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	parser.Usage = "[OPTIONS] [cmd [args...]]"

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	if !opts.Debug {
		log.SetOutput(ioutil.Discard)
	}

	if opts.Version {
		if err := version.ShowVersion(); err != nil {
			return fatal("%s", err)
		}
		return 0
	}

	if err := opts.Execute(args); err != nil {
		return fatal("%s", err)
	}

	return 0
}

package proxy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	flags "github.com/jessevdk/go-flags"
	shellwords "github.com/mattn/go-shellwords"

	"wharfr.at/wharfrat/lib/cmd/wharfrat"
	"wharfr.at/wharfrat/lib/version"
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

func Main(name string) int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	parser.Usage = "[OPTIONS] [cmd [args...]]"

	args := append([]string{name}, os.Args[1:]...)

	options, err := shellwords.Parse(os.Getenv("WHARFRAT_OPTIONS"))
	if err != nil {
		return fatal("%s", err)
	}

	_, err = parser.ParseArgs(options)
	if flagErr := (*flags.Error)(nil); errors.As(err, &flagErr) && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	if !opts.Debug {
		log.SetOutput(io.Discard)
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

package exec

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/exec"
	"wharfr.at/wharfrat/lib/version"

	flags "github.com/jessevdk/go-flags"
	shellwords "github.com/mattn/go-shellwords"
)

type options struct {
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

	parser.Usage = "[OPTIONS] <ExecCfg-file> [args...]"

	args := os.Args[1:]

	options, err := shellwords.Parse(os.Getenv("WHARFRAT_OPTIONS"))
	if err != nil {
		return fatal("%s", err)
	}

	_, err = parser.ParseArgs(options)
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	config.Debug = opts.Debug
	log.SetPrefix("WR-EXEC: ")
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if !opts.Debug {
		log.SetOutput(ioutil.Discard)
	}

	if opts.Version {
		if err := version.ShowVersion(); err != nil {
			return fatal("%s", err)
		}
		return 0
	}

	if len(args) < 1 {
		return fatal("Missing exec-file")
	}

	name, args := args[0], args[1:]

	cfg, err := exec.Parse(name)
	if err != nil {
		return fatal("Failed to parse file %s: %s", name, err)
	}

	log.Printf("EXECUTE: %s", args)
	ret, err := cfg.Execute(args)
	log.Printf("STATUS: %d %v", ret, err)
	if err != nil {
		return fatal("Failed to execute file %s: %s", name, err)
	}

	return ret
}

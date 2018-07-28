package exec

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"wharfr.at/wharfrat/lib/config"
	"wharfr.at/wharfrat/lib/docker"
	"wharfr.at/wharfrat/lib/version"

	"github.com/burntsushi/toml"
	flags "github.com/jessevdk/go-flags"
	shellwords "github.com/mattn/go-shellwords"
)

type ExecCfg struct {
	Args    []string `toml:"args"`
	Command []string `toml:"command"`
	Crate   string   `toml:"crate"`
	Project string   `toml:"project"`
	User    string   `toml:"user"`
	path    string
	meta    toml.MetaData
}

func parse(path string) (*ExecCfg, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var cfg ExecCfg
	md, err := toml.DecodeFile(absPath, &cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("Unknown config keys: %s", md.Undecoded())
	log.Printf("ExecCfg File: %s", absPath)
	log.Printf("ExecCfg: %#v", cfg)
	cfg.path = absPath
	cfg.meta = md
	return &cfg, nil
}

func (e *ExecCfg) getCrate(ls config.LabelSource) (*config.Crate, error) {
	path := e.Project
	base := filepath.Dir(e.path)
	if path == "" {
		return config.GetCrate(base, e.Crate, ls)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return config.OpenCrate(path, e.Crate, ls)
}

func (e *ExecCfg) Execute(args []string) (int, error) {
	client, err := docker.Connect()
	if err != nil {
		return 0, err
	}
	defer client.Close()
	crate, err := e.getCrate(client)
	if err != nil {
		return 0, err
	}
	log.Printf("CRATE: %#v", crate)
	container, err := client.EnsureRunning(crate, false)
	if err != nil {
		return 1, fmt.Errorf("Failed to run container: %s", err)
	}
	cmd := e.Command
	if len(cmd) == 0 {
		name := filepath.Base(e.path)
		cmd = []string{name}
	}
	if e.meta.IsDefined("args") {
		args = e.Args
	}
	cmd = append(cmd, args...)
	return client.ExecCmd(container, cmd, crate, e.User, "")
}

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

	cfg, err := parse(name)
	if err != nil {
		return fatal("Failed to parse file %s: %s", name, err)
	}

	ret, err := cfg.Execute(args)
	if err != nil {
		return fatal("Failed to execute file %s: %s", name, err)
	}

	return ret
}

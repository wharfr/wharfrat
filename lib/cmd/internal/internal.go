package internal

import (
	"errors"
	"fmt"
	"log"
	"os/exec"

	"wharfr.at/wharfrat/lib/config"

	flags "github.com/jessevdk/go-flags"
)

type options struct {
	Complete `command:"complete"`
	Proxy    `command:"proxy"`
	Exec     `command:"exec"`
	Server   `command:"server"`
	Setup    `command:"setup"`
	Homedir  `command:"homedir"`
	Search   `command:"search"`
	Version  `command:"version"`
	Debug    bool `short:"d" long:"debug" description:"Show debug output"`
}

func Main() int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)

	parser.CommandHandler = func(cmd flags.Commander, args []string) error {
		config.SetupLogging(opts.Debug)

		if cmd == nil {
			return nil
		}

		return cmd.Execute(args)
	}

	_, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		fmt.Println(err)
		return 0
	} else if x := (*exec.ExitError)(nil); errors.As(err, &x) && x.ProcessState.Exited() {
		log.Printf("COMMAND EXITED: %d", x.ProcessState.ExitCode())
		return x.ProcessState.ExitCode()
	} else if err != nil {
		return 1
	}

	return 0
}

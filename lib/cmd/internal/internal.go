package internal

import (
	"io/ioutil"
	"log"

	flags "github.com/jessevdk/go-flags"
)

type options struct {
	Complete `command:"complete"`
	Proxy    `command:"proxy"`
	Server   `command:"server"`
	Setup    `command:"setup"`
	Homedir  `command:"homedir"`
	Version  `command:"version"`
	Debug    bool `short:"d" long:"debug" description:"Show debug output"`
}

func Main() int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	parser.CommandHandler = func(cmd flags.Commander, args []string) error {
		if !opts.Debug {
			log.SetOutput(ioutil.Discard)
		}

		if cmd == nil {
			return nil
		}

		return cmd.Execute(args)
	}

	_, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	return 0
}

package wharfrat

import (
	"io/ioutil"
	"log"

	flags "github.com/jessevdk/go-flags"
)

type options struct {
	List   `command:"list" description:"List existing containers"`
	Run    `command:"run" description:"Run a command in a container"`
	Start  `command:"start" description:"Start an existing container"`
	Stop   `command:"stop" description:"Stop an existing container"`
	Remove `command:"remove" description:"Remove an existing container"`
	Rm     Remove `command:"rm" description:"Remove an existing container"`
	Login  `command:"login" description:"Cache credentials for a registry"`
	Logout `command:"logout" description:"Drop credentials for a registry"`
	Debug  bool `short:"d" long:"debug" description:"Show debug output"`
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

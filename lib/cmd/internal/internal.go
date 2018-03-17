package internal

import (
	"log"

	flags "github.com/jessevdk/go-flags"
)

type options struct {
	Proxy  `command:"proxy"`
	Server `command:"server"`
}

func Main() int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}
	log.Printf("Internal Args: %#v, Opts: %#v", args, opts)

	return 0
}

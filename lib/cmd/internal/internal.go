package internal

import (
	flags "github.com/jessevdk/go-flags"
)

type options struct {
	Proxy  `command:"proxy"`
	Server `command:"server"`
}

func Main() int {
	opts := options{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	_, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	return 0
}

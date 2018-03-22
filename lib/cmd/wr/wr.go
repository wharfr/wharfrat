package wr

import (
	"fmt"
	"os"

	"git.qur.me/qur/wharf_rat/lib/cmd/wharfrat"

	flags "github.com/jessevdk/go-flags"
)

func fatal(msg string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	return 1
}

func Main() int {
	opts := wharfrat.Run{}

	parser := flags.NewParser(&opts, flags.Default|flags.PassAfterNonOption)

	args, err := parser.Parse()
	if flagErr, ok := err.(*flags.Error); ok && flagErr.Type == flags.ErrHelp {
		return 0
	} else if err != nil {
		return 1
	}

	if err := opts.Execute(args); err != nil {
		return fatal("%s", err)
	}

	return 0
}

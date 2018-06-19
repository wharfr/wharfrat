package cmd

import (
	"os"
	"path/filepath"

	"wharfr.at/wharfrat/lib/cmd/internal"
	"wharfr.at/wharfrat/lib/cmd/wharfrat"
	"wharfr.at/wharfrat/lib/cmd/wr"
)

func Main() int {
	switch filepath.Base(os.Args[0]) {
	case "wr":
		return wr.Main()
	case "wr-init":
		return internal.Main()
	default:
		return wharfrat.Main()
	}
}

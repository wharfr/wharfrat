package cmd

import (
	"os"
	"path/filepath"

	"wharfr.at/wharfrat/lib/cmd/internal"
	"wharfr.at/wharfrat/lib/cmd/proxy"
	"wharfr.at/wharfrat/lib/cmd/wharfrat"
	"wharfr.at/wharfrat/lib/cmd/wr"
)

func Main() int {
	switch name := filepath.Base(os.Args[0]); name {
	case "wr":
		return wr.Main()
	case "wr-init":
		return internal.Main()
	case "wharfrat":
		return wharfrat.Main()
	default:
		return proxy.Main(name)
	}
}

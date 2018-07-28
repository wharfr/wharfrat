package cmd

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"wharfr.at/wharfrat/lib/cmd/exec"
	"wharfr.at/wharfrat/lib/cmd/internal"
	"wharfr.at/wharfrat/lib/cmd/proxy"
	"wharfr.at/wharfrat/lib/cmd/wharfrat"
	"wharfr.at/wharfrat/lib/cmd/wr"
)

func Main() int {
	name := filepath.Base(os.Args[0])
	if strings.HasPrefix(name, "wr-") {
		switch name[3:] {
		case "exec":
			return exec.Main()
		case "init":
			return internal.Main()
		default:
			log.Printf("ERROR: Unknown alias: %s", name)
			return 1
		}
	}
	switch name {
	case "wr":
		return wr.Main()
	case "wharfrat":
		return wharfrat.Main()
	default:
		return proxy.Main(name)
	}
}

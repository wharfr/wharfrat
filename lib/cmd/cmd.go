package cmd

import (
	"os"
	"path/filepath"

	"git.qur.me/qur/wharf_rat/lib/cmd/internal"
	"git.qur.me/qur/wharf_rat/lib/cmd/wharfrat"
	"git.qur.me/qur/wharf_rat/lib/cmd/wr"
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

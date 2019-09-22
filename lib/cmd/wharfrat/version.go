package wharfrat

import (
	"fmt"

	"wharfr.at/wharfrat/lib/version"
)

type Version struct{
	Commit bool `long:"commit" description:"Only show the git commit"`
}

func (v *Version) Execute(args []string) error {
	if v.Commit {
		fmt.Println(version.Commit())
		return nil
	}
	return version.ShowVersion()
}

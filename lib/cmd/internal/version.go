package internal

import "wharfr.at/wharfrat/lib/version"

type Version struct{}

func (v *Version) Execute(args []string) error {
	return version.ShowVersion()
}

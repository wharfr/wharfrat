package wharfrat

import "git.qur.me/qur/wharf_rat/lib/version"

type Version struct{}

func (v *Version) Execute(args []string) error {
	return version.ShowVersion()
}

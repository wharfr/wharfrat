package label

const domain = "at.wharfr.wharfrat"

// Labels intended for use on containers
const (
	Project   = domain + ".project"
	Crate     = domain + ".crate"
	Commit    = domain + ".commit"
	Config    = domain + ".config"
	Branch    = domain + ".branch"
	User      = domain + ".user"
	Namespace = domain + ".namespace"
)

// Labels intended for use on images
const (
	Shell = domain + ".shell"
)

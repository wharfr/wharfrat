package label

const domain = "at.wharfr.wharfrat"

// Labels intended for use on containers
const (
	Project = domain + ".project"
	Crate   = domain + ".crate"
	Config  = domain + ".config"
	Branch  = domain + ".branch"
)

// Labels intended for use on images
const (
	Shell = domain + ".shell"
)

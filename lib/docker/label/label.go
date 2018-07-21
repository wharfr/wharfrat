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

const (
	oldDomain  = "me.qur.wharf-rat"
	OldProject = oldDomain + ".project"
	oldCrate   = oldDomain + ".crate"
	oldConfig  = oldDomain + ".config"
	oldBranch  = oldDomain + ".branch"
)

// FixOld replaces old labels in the given map, with the new equivalents
func FixOld(labels map[string]string) {
	project, found := labels[OldProject]
	if !found {
		return
	}
	labels[Project] = project
	labels[Crate] = labels[oldCrate]
	labels[Config] = labels[oldConfig]
	labels[Branch] = labels[oldBranch]
	delete(labels, OldProject)
	delete(labels, oldCrate)
	delete(labels, oldConfig)
	delete(labels, oldBranch)
}

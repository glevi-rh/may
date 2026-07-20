package pod

// TODO(@konflux-ci): replace this hardcoded list with a configurable source
// so adding new local flavors
// does not require a code change and redeploy.
var localFlavors = map[string]struct{}{
	"localhost": {},
	"local":     {},
}

func IsLocalFlavor(flavor string) bool {
	_, ok := localFlavors[flavor]
	return ok
}

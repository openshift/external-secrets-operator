package tlsprofile

import (
	"sync"

	configv1 "github.com/openshift/api/config/v1"
)

// startupProfile caches the cluster TLS security profile resolved once during
// operator startup. The operator restarts whenever
// apiserver.config.openshift.io/cluster changes its TLS profile or adherence
// policy (see the TLS security profile watcher wired up in main), so the value
// read at startup stays valid for the lifetime of the process and controllers
// can reuse it instead of re-reading the APIServer object on every reconcile.
var startupProfile struct {
	sync.RWMutex
	spec *configv1.TLSProfileSpec
}

// SetStartupProfile records the honored cluster TLS profile resolved at
// startup. A nil spec means no cluster profile is enforced and existing TLS
// settings must be left unchanged.
func SetStartupProfile(spec *configv1.TLSProfileSpec) {
	startupProfile.Lock()
	defer startupProfile.Unlock()
	startupProfile.spec = spec.DeepCopy()
}

// StartupProfile returns a copy of the profile recorded by SetStartupProfile,
// or nil when no cluster TLS profile is to be enforced.
func StartupProfile() *configv1.TLSProfileSpec {
	startupProfile.RLock()
	defer startupProfile.RUnlock()
	return startupProfile.spec.DeepCopy()
}

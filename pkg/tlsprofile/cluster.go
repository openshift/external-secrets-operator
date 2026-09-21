package tlsprofile

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
)

// APIServerClusterName is the singleton apiserver.config.openshift.io object.
const APIServerClusterName = "cluster"

// FetchAPIServerFunc retrieves apiserver.config.openshift.io/cluster.
type FetchAPIServerFunc func(ctx context.Context) (*configv1.APIServer, error)

// FetchErrorMode controls how ResolveHonoredTLSProfile treats fetch failures.
type FetchErrorMode int

const (
	// FetchErrorPropagateExceptNotFound returns NotFound as a soft skip and
	// propagates all other fetch errors.
	FetchErrorPropagateExceptNotFound FetchErrorMode = iota
)

// ObjectGetter is the Get subset used to fetch APIServer.
type ObjectGetter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
}

// clientReaderAdapter wraps a controller-runtime client.Reader (which has
// variadic GetOption) so it satisfies ObjectGetter.
type clientReaderAdapter struct {
	reader client.Reader
}

func (a *clientReaderAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return a.reader.Get(ctx, key, obj)
}

// NewClientReaderObjectGetter adapts a controller-runtime client.Reader to the
// ObjectGetter interface.
func NewClientReaderObjectGetter(r client.Reader) ObjectGetter {
	return &clientReaderAdapter{reader: r}
}

// NewClientReaderAPIServerFetch returns a controller-runtime fetcher for the
// cluster APIServer.
func NewClientReaderAPIServerFetch(r ObjectGetter) FetchAPIServerFunc {
	return func(ctx context.Context) (*configv1.APIServer, error) {
		apiServer := &configv1.APIServer{}
		if err := r.Get(ctx, types.NamespacedName{Name: APIServerClusterName}, apiServer); err != nil {
			return nil, err
		}
		return apiServer, nil
	}
}

// ResolveHonoredTLSProfile fetches the cluster APIServer via fetch and, when
// tlsAdherence requires enforcement, returns EffectiveSpec. A nil profile with
// a nil error means the caller should leave existing TLS settings unchanged.
func ResolveHonoredTLSProfile(ctx context.Context, fetch FetchAPIServerFunc, component string, mode FetchErrorMode) (*configv1.TLSProfileSpec, error) {
	if fetch == nil {
		return nil, fmt.Errorf("APIServer fetch function is nil")
	}

	apiServer, err := fetch(ctx)
	if err != nil {
		switch mode {
		case FetchErrorPropagateExceptNotFound:
			if apierrors.IsNotFound(err) {
				klog.V(4).Infof("skipping cluster TLS profile for %s: apiserver.config.openshift.io/cluster not found", component)
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get apiserver.config.openshift.io/cluster: %w", err)
		default:
			return nil, fmt.Errorf("failed to get apiserver.config.openshift.io/cluster: %w", err)
		}
	}

	adherence := apiServer.Spec.TLSAdherence
	if !libgocrypto.ShouldHonorClusterTLSProfile(adherence) {
		klog.V(4).Infof("skipping cluster TLS profile for %s: apiserver tlsAdherence=%q", component, adherence)
		return nil, nil
	}
	if adherence != configv1.TLSAdherencePolicyStrictAllComponents {
		klog.Warningf("apiserver.config.openshift.io/cluster has unknown tlsAdherence %q; treating as StrictAllComponents for %s", adherence, component)
	}

	return EffectiveSpec(apiServer.Spec.TLSSecurityProfile)
}

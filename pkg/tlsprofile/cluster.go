package tlsprofile

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// APIServerClusterName is the singleton apiserver.config.openshift.io object.
const APIServerClusterName = "cluster"

// FetchAPIServerFunc retrieves apiserver.config.openshift.io/cluster.
type FetchAPIServerFunc func(ctx context.Context) (*configv1.APIServer, error)

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

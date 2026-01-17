package crd_annotator

import (
	"context"
	"testing"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr/testr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

// testReconciler returns a sample Reconciler instance.
func testReconciler(t *testing.T) *Reconciler {
	return &Reconciler{
		ctx: context.Background(),
		log: testr.New(t),
	}
}

// testExtendExternalSecretsConfig enables CRD annotation specific configs on existing externalsecretsconfig object.
func testExtendExternalSecretsConfig(esc *operatorv1alpha1.ExternalSecretsConfig) {
	esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
		ControllerConfig: operatorv1alpha1.ControllerConfig{
			CertProvider: &operatorv1alpha1.CertProvidersConfig{
				CertManager: &operatorv1alpha1.CertManagerConfig{
					InjectAnnotations: "true",
				},
			},
		},
	}
}

// testCRD is for generating a sample CRD object for tests.
func testCRD() *crdv1.CustomResourceDefinition {
	return &crdv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: commontest.TestCRDName,
			Annotations: map[string]string{
				"testAnnotation": "true",
			},
		},
		Spec: crdv1.CustomResourceDefinitionSpec{
			Group: "operator.openshift.io",
		},
	}
}

func TestReconcile(t *testing.T) {
	tests := []struct {
		name                    string
		request                 ctrl.Request
		preReq                  func(*Reconciler, *fakes.FakeCtrlClient)
		expectedStatusCondition []metav1.Condition
		wantErr                 string
	}{
		{
			name: "reconciliation successful for a specific CRD",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionTrue,
					Reason: operatorv1alpha1.ReasonCompleted,
				},
			},
		},
		{
			name: "reconciliation successful for all CRDs",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: reconcileObjectIdentifier,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
				m.ListCalls(func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
					if o, ok := obj.(*crdv1.CustomResourceDefinitionList); ok {
						crdList := &crdv1.CustomResourceDefinitionList{}
						crdList.Items = []crdv1.CustomResourceDefinition{
							*testCRD(),
						}
						crdList.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionTrue,
					Reason: operatorv1alpha1.ReasonCompleted,
				},
			},
		},
		{
			name: "reconciliation fails when fetching externalsecrets",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						return commontest.ErrTestClient
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionFalse,
					Reason: operatorv1alpha1.ReasonFailed,
				},
			},
			wantErr: `failed to fetch externalsecretsconfigs.operator.openshift.io "/cluster" during reconciliation: test client error`,
		},
		{
			name: "reconciliation successful externalsecretsconfigs does not exist",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						return errors.NewNotFound(schema.GroupResource{
							Group:    operatorv1alpha1.GroupVersion.Group,
							Resource: "externalsecretsconfigs",
						}, commontest.TestExternalSecretsConfigResourceName)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{},
		},
		{
			name: "reconciliation successful config disabled",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{},
		},
		{
			name: "reconciliation fails while listing CRD",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: reconcileObjectIdentifier,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						return commontest.ErrTestClient
					}
					return nil
				})
				m.ListCalls(func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
					return commontest.ErrTestClient
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionFalse,
					Reason: operatorv1alpha1.ReasonFailed,
				},
			},
			wantErr: `failed while updating annotations in all CRDs: failed to list managed CRD resources: test client error`,
		},
		{
			name: "reconciliation successful no required CRDs exist",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: reconcileObjectIdentifier,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						return commontest.ErrTestClient
					}
					return nil
				})
				m.ListCalls(func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
					if o, ok := obj.(*crdv1.CustomResourceDefinitionList); ok {
						crdList := &crdv1.CustomResourceDefinitionList{}
						crdList.Items = []crdv1.CustomResourceDefinition{}
						crdList.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionTrue,
					Reason: operatorv1alpha1.ReasonCompleted,
				},
			},
		},
		{
			name: "reconciliation fails while fetching CRD",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						return commontest.ErrTestClient
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionFalse,
					Reason: operatorv1alpha1.ReasonFailed,
				},
			},
			wantErr: `failed to fetch customresourcedefinitions.apiextensions.k8s.io "/test-crd" during reconciliation: test client error`,
		},
		{
			name: "reconciliation fails when CRD not found",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						return errors.NewNotFound(schema.GroupResource{
							Group:    crdv1.SchemeGroupVersion.Group,
							Resource: commontest.TestCRDName,
						}, commontest.TestCRDName)
					}
					return nil
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionFalse,
					Reason: operatorv1alpha1.ReasonFailed,
				},
			},
			wantErr: `failed to fetch customresourcedefinitions.apiextensions.k8s.io "/test-crd" during reconciliation: test-crd.apiextensions.k8s.io "test-crd" not found`,
		},
		{
			name: "reconciliation fails during annotation patch",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
				m.PatchCalls(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					return commontest.ErrTestClient
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionTrue,
					Reason: operatorv1alpha1.ReasonCompleted,
				},
			},
			wantErr: `failed to update annotations in "/test-crd": test client error`,
		},
		{
			name: "reconciliation fails while updating status",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: commontest.TestCRDName,
				},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := commontest.TestExternalSecretsConfig()
						testExtendExternalSecretsConfig(esc)
						esc.DeepCopyInto(o)
					case *crdv1.CustomResourceDefinition:
						crd := testCRD()
						crd.DeepCopyInto(o)
					}
					return nil
				})
				m.StatusUpdateCalls(func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return commontest.ErrTestClient
				})
			},
			expectedStatusCondition: []metav1.Condition{
				{
					Type:   operatorv1alpha1.UpdateAnnotation,
					Status: metav1.ConditionTrue,
					Reason: operatorv1alpha1.ReasonCompleted,
				},
			},
			wantErr: `failed to update externalsecretsconfigs.operator.openshift.io "/cluster" status: test client error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.CtrlClient = mock
			_, err := r.Reconcile(context.Background(), tt.request)

			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("Reconcile() err: %v, wantErr: %v", err, tt.wantErr)
			}
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			key := types.NamespacedName{
				Name: common.ExternalSecretsConfigObjectName,
			}
			_ = r.Get(r.ctx, key, esc)
			for _, c1 := range esc.Status.Conditions {
				for _, c2 := range tt.expectedStatusCondition {
					if c1.Type == c2.Type {
						if c1.Status != c2.Status || c1.Reason != c2.Reason {
							t.Errorf("Reconcile() condition: %+v, expectedStatusCondition: %+v", c1, c2)
						}
					}
				}
			}
		})
	}
}

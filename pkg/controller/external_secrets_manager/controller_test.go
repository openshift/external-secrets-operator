/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package external_secrets_manager

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// testReconciler returns a sample Reconciler instance.
func testReconciler(t *testing.T) *Reconciler {
	return &Reconciler{
		Scheme:        runtime.NewScheme(),
		ctx:           context.Background(),
		eventRecorder: record.NewFakeRecorder(100),
		log:           testr.New(t),
		now:           &common.Now{},
	}
}

func TestReconcile(t *testing.T) {
	var esm *operatorv1alpha1.ExternalSecretsManager

	tests := []struct {
		name                    string
		preReq                  func(*Reconciler, *fakes.FakeCtrlClient)
		expectedStatusCondition []operatorv1alpha1.ControllerStatus
		wantErr                 string
	}{
		{
			name: "esm reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, option ...client.UpdateOption) error {
					if o, ok := obj.(*operatorv1alpha1.ExternalSecretsManager); ok {
						o.DeepCopyInto(esm)
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := &operatorv1alpha1.ExternalSecretsConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsConfigObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsConfigSpec{},
							Status: operatorv1alpha1.ExternalSecretsConfigStatus{
								ConditionalStatus: operatorv1alpha1.ConditionalStatus{
									Conditions: []metav1.Condition{
										{
											Type:    operatorv1alpha1.Ready,
											Status:  metav1.ConditionTrue,
											Message: "test ready",
										},
										{
											Type:    operatorv1alpha1.Degraded,
											Status:  metav1.ConditionFalse,
											Message: "",
										},
									},
								},
							},
						}
						esc.DeepCopyInto(o)
					case *operatorv1alpha1.ExternalSecretsManager:
						esmObj := &operatorv1alpha1.ExternalSecretsManager{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsManagerObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsManagerSpec{},
						}
						esmObj.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []operatorv1alpha1.ControllerStatus{
				{
					Name: externalSecretsControllerId,
					Conditions: []operatorv1alpha1.Condition{
						{
							Type:    operatorv1alpha1.Ready,
							Status:  metav1.ConditionTrue,
							Message: "test ready",
						},
						{
							Type:    operatorv1alpha1.Degraded,
							Status:  metav1.ConditionFalse,
							Message: "",
						},
					},
				},
			},
		},
		{
			name: "esm object not found",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if _, ok := obj.(*operatorv1alpha1.ExternalSecretsManager); ok {
						return errors.NewNotFound(operatorv1alpha1.Resource("externalsecretsmanagers"), ns.Name)
					}
					return nil
				})
			},
			expectedStatusCondition: []operatorv1alpha1.ControllerStatus{},
			wantErr:                 `failed to fetch externalsecretsmanagers.operator.openshift.io "/cluster" during reconciliation: externalsecretsmanagers.operator.openshift.io "cluster" not found`,
		},
		{
			name: "externalsecretsconfig object not found",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsManager:
						esmObj := &operatorv1alpha1.ExternalSecretsManager{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsManagerObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsManagerSpec{},
						}
						esmObj.DeepCopyInto(o)
					case *operatorv1alpha1.ExternalSecretsConfig:
						return errors.NewNotFound(operatorv1alpha1.Resource("externalsecretsconfigs"), ns.Name)
					}
					return nil
				})
			},
			expectedStatusCondition: []operatorv1alpha1.ControllerStatus{},
		},
		{
			name: "externalsecretsconfig fetch fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsManager:
						esmObj := &operatorv1alpha1.ExternalSecretsManager{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsManagerObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsManagerSpec{},
						}
						esmObj.DeepCopyInto(o)
					case *operatorv1alpha1.ExternalSecretsConfig:
						return errors.NewServerTimeout(operatorv1alpha1.Resource("externalsecretsconfig"), "Get", 5)
					}
					return nil
				})
			},
			expectedStatusCondition: []operatorv1alpha1.ControllerStatus{},
			wantErr:                 `failed to fetch externalsecretsconfigs.operator.openshift.io "/cluster" during reconciliation: The Get operation against externalsecretsconfig.operator.openshift.io could not be completed at this time, please try again.`,
		},
		{
			name: "esm reconciliation successful with new conditions",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, option ...client.UpdateOption) error {
					if o, ok := obj.(*operatorv1alpha1.ExternalSecretsManager); ok {
						o.DeepCopyInto(esm)
					}
					return nil
				})
				m.StatusUpdateCalls(func(ctx context.Context, obj client.Object, option ...client.SubResourceUpdateOption) error {
					if o, ok := obj.(*operatorv1alpha1.ExternalSecretsManager); ok {
						o.DeepCopyInto(esm)
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *operatorv1alpha1.ExternalSecretsConfig:
						esc := &operatorv1alpha1.ExternalSecretsConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsConfigObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsConfigSpec{},
							Status: operatorv1alpha1.ExternalSecretsConfigStatus{
								ConditionalStatus: operatorv1alpha1.ConditionalStatus{
									Conditions: []metav1.Condition{
										{
											Type:    operatorv1alpha1.Ready,
											Status:  metav1.ConditionTrue,
											Message: "test ready",
										},
										{
											Type:    operatorv1alpha1.Degraded,
											Status:  metav1.ConditionFalse,
											Message: "",
										},
									},
								},
							},
						}
						esc.DeepCopyInto(o)
					case *operatorv1alpha1.ExternalSecretsManager:
						esmObj := &operatorv1alpha1.ExternalSecretsManager{
							ObjectMeta: metav1.ObjectMeta{
								Name: common.ExternalSecretsManagerObjectName,
							},
							Spec: operatorv1alpha1.ExternalSecretsManagerSpec{},
							Status: operatorv1alpha1.ExternalSecretsManagerStatus{
								ControllerStatuses: []operatorv1alpha1.ControllerStatus{
									{
										Name: externalSecretsControllerId,
										Conditions: []operatorv1alpha1.Condition{
											{
												Type:    operatorv1alpha1.Ready,
												Status:  metav1.ConditionFalse,
												Message: "",
											},
											{
												Type:    operatorv1alpha1.Degraded,
												Status:  metav1.ConditionFalse,
												Message: "",
											},
										},
									},
								},
							},
						}
						esmObj.DeepCopyInto(o)
					}
					return nil
				})
			},
			expectedStatusCondition: []operatorv1alpha1.ControllerStatus{
				{
					Name: externalSecretsControllerId,
					Conditions: []operatorv1alpha1.Condition{
						{
							Type:    operatorv1alpha1.Ready,
							Status:  metav1.ConditionTrue,
							Message: "test ready",
						},
						{
							Type:    operatorv1alpha1.Degraded,
							Status:  metav1.ConditionFalse,
							Message: "",
						},
					},
				},
			},
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
			esm = &operatorv1alpha1.ExternalSecretsManager{}
			_, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: common.ExternalSecretsManagerObjectName,
				},
			})

			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("Reconcile() err: %v, wantErr: %v", err, tt.wantErr)
			}
			for _, c1 := range esm.Status.ControllerStatuses {
				for _, c2 := range tt.expectedStatusCondition {
					if c1.Name != c2.Name {
						continue
					}

					// assuming you'll already know the order of the expected conditions from before
					// given this is only a test.
					if !reflect.DeepEqual(c2.Conditions, c1.Conditions) {
						t.Errorf("Reconcile() condition: %+v, expectedStatusCondition: %+v", c1, c2)
					}
				}
			}
		})
	}
}

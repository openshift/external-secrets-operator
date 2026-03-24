//go:build e2e
// +build e2e

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

package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

const (
	artifactLogTailLines   = 500
	externalSecretsGroup   = "external-secrets.io"
	operatorOpenShiftGroup = "operator.openshift.io"
	esoV1                  = "v1"
	esoV1alpha1            = "v1alpha1"
)

// DumpE2EArtifacts writes logs, pod describes, events, and ESO resources when a test fails.
// Call from AfterEach when CurrentSpecReport().Failed(). outputDir is the base directory (e.g. getTestDir(): ARTIFACT_DIR in CI, or repo _output when running locally). Dump is written to outputDir/e2e-artifacts/failure-<timestamp>/.
func DumpE2EArtifacts(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, operatorNamespace, operandNamespace, testNamespace, outputDir string) error {
	if outputDir == "" {
		return nil
	}
	ts := time.Now().Format("20060102-150405")
	base := filepath.Join(outputDir, "e2e-artifacts", fmt.Sprintf("failure-%s", ts))
	if err := os.MkdirAll(base, 0755); err != nil {
		return fmt.Errorf("mkdir e2e-artifacts: %w", err)
	}

	namespaces := []string{operatorNamespace, operandNamespace}
	if testNamespace != "" {
		namespaces = append(namespaces, testNamespace)
	}

	// Pod logs and describes
	podsDir := filepath.Join(base, "pods")
	_ = os.MkdirAll(podsDir, 0755)
	for _, ns := range namespaces {
		podList, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			writeFile(filepath.Join(podsDir, ns+"_list_error.txt"), []byte(err.Error()))
			continue
		}
		for _, pod := range podList.Items {
			name := pod.Name
			// Logs
			opts := &corev1.PodLogOptions{TailLines: int64Ptr(artifactLogTailLines)}
			req := clientset.CoreV1().Pods(ns).GetLogs(name, opts)
			logBytes, err := req.DoRaw(ctx)
			if err != nil {
				writeFile(filepath.Join(podsDir, ns+"_"+name+".log"), []byte(fmt.Sprintf("failed to get logs: %v\n", err)))
			} else {
				writeFile(filepath.Join(podsDir, ns+"_"+name+".log"), logBytes)
			}
			// Describe
			podDesc, err := clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				writeFile(filepath.Join(podsDir, ns+"_"+name+"_describe.txt"), []byte(err.Error()))
				continue
			}
			descYaml, _ := yaml.Marshal(podDesc)
			writeFile(filepath.Join(podsDir, ns+"_"+name+"_describe.yaml"), descYaml)
		}
	}

	// Events per namespace
	eventsDir := filepath.Join(base, "events")
	_ = os.MkdirAll(eventsDir, 0755)
	for _, ns := range namespaces {
		evList, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{Limit: 500})
		if err != nil {
			writeFile(filepath.Join(eventsDir, ns+"_events.txt"), []byte(err.Error()))
			continue
		}
		var b strings.Builder
		for _, ev := range evList.Items {
			b.WriteString(fmt.Sprintf("%s %s %s: %s\n", ev.LastTimestamp.Format(time.RFC3339), ev.InvolvedObject.Kind, ev.InvolvedObject.Name, ev.Message))
		}
		writeFile(filepath.Join(eventsDir, ns+"_events.txt"), []byte(b.String()))
	}

	// ESO resources (ClusterSecretStore, ExternalSecret, PushSecret) and operator ExternalSecretsConfig
	resDir := filepath.Join(base, "resources")
	_ = os.MkdirAll(resDir, 0755)
	gr, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		writeFile(filepath.Join(resDir, "discovery_error.txt"), []byte(err.Error()))
	} else {
		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		// ExternalSecretsConfig (operator.openshift.io, cluster-scoped)
		dumpESOResource(ctx, dynamicClient, mapper, resDir, operatorOpenShiftGroup, esoV1alpha1, "ExternalSecretsConfig", "")
		// ClusterSecretStores (cluster-scoped)
		dumpESOResource(ctx, dynamicClient, mapper, resDir, externalSecretsGroup, esoV1, "ClusterSecretStore", "")
		// ExternalSecrets and PushSecrets in operand and test namespace
		for _, ns := range []string{operandNamespace, testNamespace} {
			if ns == "" {
				continue
			}
			dumpESOResource(ctx, dynamicClient, mapper, resDir, externalSecretsGroup, esoV1, "ExternalSecret", ns)
			dumpESOResource(ctx, dynamicClient, mapper, resDir, externalSecretsGroup, esoV1alpha1, "PushSecret", ns)
		}
	}

	return nil
}

func dumpESOResource(ctx context.Context, dynamicClient dynamic.Interface, mapper meta.RESTMapper, resDir, group, version, kind, namespace string) {
	gv := schema.GroupVersion{Group: group, Version: version}
	gk := schema.GroupKind{Group: group, Kind: kind}
	mapping, err := mapper.RESTMapping(gk, gv.Version)
	if err != nil {
		writeFile(filepath.Join(resDir, kind+"_mapping_error.txt"), []byte(err.Error()))
		return
	}
	var list *unstructured.UnstructuredList
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		list, err = dynamicClient.Resource(mapping.Resource).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dynamicClient.Resource(mapping.Resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		writeFile(filepath.Join(resDir, kind+"_"+namespace+"_list_error.txt"), []byte(err.Error()))
		return
	}
	outName := kind
	if namespace != "" {
		outName = kind + "_" + namespace
	}
	outName = sanitizeFilename(outName) + ".yaml"
	var b strings.Builder
	for _, item := range list.Items {
		bb, _ := yaml.Marshal(item.Object)
		b.Write(bb)
		b.WriteString("---\n")
	}
	writeFile(filepath.Join(resDir, outName), []byte(b.String()))
}

func sanitizeFilename(s string) string {
	s = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(s, "_")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

func writeFile(path string, data []byte) {
	_ = os.WriteFile(path, data, 0644)
}

func int64Ptr(n int) *int64 {
	v := int64(n)
	return &v
}

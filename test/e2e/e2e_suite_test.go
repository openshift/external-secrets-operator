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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var (
	cfg                *rest.Config
	suiteClientset     *kubernetes.Clientset
	suiteDynamicClient *dynamic.DynamicClient
	suiteRuntimeClient client.Client
)

func getTestDir() string {
	if os.Getenv("OPENSHIFT_CI") == "true" {
		if d := os.Getenv("ARTIFACT_DIR"); d != "" {
			return d
		}
	}
	if d := os.Getenv("ARTIFACT_DIR"); d != "" {
		return d
	}
	// Local run: use repo _output.
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Clean(filepath.Join(cwd, "..", "_output"))
	}
	return "/tmp"
}

var _ = BeforeSuite(func() {
	var err error

	By("Initializing Kubernetes config")

	cfg, err = config.GetConfig()
	Expect(err).NotTo(HaveOccurred(), "failed to get kubeconfig")

	By("Creating suite Kubernetes clients")
	suiteClientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "failed to create clientset")
	suiteDynamicClient, err = dynamic.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "failed to create dynamic client")
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	suiteRuntimeClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred(), "failed to create runtime client")
})

var _ = AfterSuite(func() {
	By("Cleaning up ESO operand and related resources (operand CR instances, cluster ExternalSecretsConfig, namespace, clusterroles, webhooks)")
	utils.CleanupESOOperandAndRelated(context.Background(), cfg)
})

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting external-secrets-operator suite\n")

	suiteConfig, reportConfig := GinkgoConfiguration()

	// Suite behavior: longer timeout, run all specs after first failure for easier debugging.
	suiteConfig.Timeout = 90 * time.Minute
	suiteConfig.FailFast = false
	suiteConfig.FlakeAttempts = 0
	suiteConfig.MustPassRepeatedly = 1

	testDir := getTestDir()
	reportConfig.JSONReport = filepath.Join(testDir, "e2e-report.json")
	reportConfig.JUnitReport = filepath.Join(testDir, "e2e-junit.xml")
	reportConfig.NoColor = true
	// Verbosity is left to the Makefile (-ginkgo.v) to avoid conflicting with -v/-vv/--succinct.
	reportConfig.ShowNodeEvents = true
	reportConfig.FullTrace = true
	reportConfig.SilenceSkips = true

	RunSpecs(t, "e2e suite", suiteConfig, reportConfig)
}

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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func NewClientsConfigForTest(t *testing.T) (kubernetes.Interface, dynamic.Interface) {
	config, err := GetConfigForTest(t)
	if err == nil {
		t.Logf("Found configuration for host %v.\n", config.Host)
	}

	require.NoError(t, err)
	kubeClient, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	dynamicKubeConfig, err := dynamic.NewForConfig(config)
	require.NoError(t, err)
	return kubeClient, dynamicKubeConfig
}

func GetConfigForTest(t *testing.T) (*rest.Config, error) {
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{ClusterInfo: api.Cluster{InsecureSkipTLSVerify: true}})
	config, err := clientConfig.ClientConfig()
	if err == nil {
		t.Logf("Found configuration for host %v.\n", config.Host)
	}

	require.NoError(t, err)
	return config, err
}

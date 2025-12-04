//go:build tools
// +build tools

// Official workaround to track tool dependencies with go modules:
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

package tools

import (
	// Makefile
	_ "github.com/elastic/crd-ref-docs"
	_ "github.com/go-bindata/go-bindata/go-bindata"
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/openshift/build-machinery-go"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "sigs.k8s.io/kube-api-linter/pkg/plugin"

	// prow-ci
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)

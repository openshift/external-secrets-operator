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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	apiPath       = "/rest/api/1"
	apiJobTimeout = 2 * time.Minute
)

// inClusterBaseURL is the Bitwarden SDK server URL as seen from inside the cluster (same as external-secrets controller). Respects BITWARDEN_SDK_SERVER_URL.
var inClusterBaseURL = utils.GetBitwardenSDKServerURL()

var _ = Describe("Bitwarden SDK Server API", Ordered, Label("API:Bitwarden", "Suite:Bitwarden"), func() {
	var (
		ctx       context.Context
		clientset *kubernetes.Clientset
		namespace string
	)

	BeforeAll(func() {
		ctx = context.Background()
		var err error
		clientset = suiteClientset
		Expect(clientset).NotTo(BeNil(), "suite clientset not initialized")

		By("Ensuring Bitwarden operand is ready (enable plugin, wait for server)")
		Expect(ensureBitwardenOperandReady(ctx)).To(Succeed())

		credNamespace := utils.BitwardenCredSecretNamespace
		_, err = clientset.CoreV1().Secrets(credNamespace).Get(ctx, utils.BitwardenCredSecretName, metav1.GetOptions{})
		if err != nil {
			Skip(fmt.Sprintf("Bitwarden credentials secret %s/%s required for API tests. See docs/e2e/README.md. Error: %v", credNamespace, utils.BitwardenCredSecretName, err))
		}

		// Run API test Jobs in the operand namespace (external-secrets) so they can reach bitwarden-sdk-server
		// (same network as the controller). Copy the cred secret there so the Job can mount it.
		namespace = utils.BitwardenOperandNamespace
		By("Copying Bitwarden cred secret to " + namespace + " for API test Jobs")
		Expect(utils.CopySecretToNamespace(ctx, clientset, utils.BitwardenCredSecretName, credNamespace, utils.BitwardenCredSecretName, namespace)).To(Succeed())
	})

	runAPIJob := func(jobName, script string) (int, string) {
		code, logs, err := utils.RunBitwardenAPIJob(ctx, clientset, namespace, jobName, []string{"sh", "-c", script}, apiJobTimeout)
		Expect(err).NotTo(HaveOccurred(), "job %s: %s", jobName, logs)
		return code, logs
	}

	Context("Health", func() {
		It("GET /ready should return 200 with body ready", func() {
			script := fmt.Sprintf("code=$(curl -k -s -o /tmp/out -w '%%{http_code}' %s/ready) && body=$(cat /tmp/out) && [ \"$code\" = \"200\" ] && [ \"$body\" = \"ready\" ] || (echo \"code=$code body=$body\"; exit 1)", inClusterBaseURL)
			code, logs := runAPIJob("api-ready-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})

		It("GET /live should return 200 with body live", func() {
			script := fmt.Sprintf("code=$(curl -k -s -o /tmp/out -w '%%{http_code}' %s/live) && body=$(cat /tmp/out) && [ \"$code\" = \"200\" ] && [ \"$body\" = \"live\" ] || (echo \"code=$code body=$body\"; exit 1)", inClusterBaseURL)
			code, logs := runAPIJob("api-live-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})
	})

	Context("Auth", func() {
		It("request without Warden-Access-Token should return 401", func() {
			script := fmt.Sprintf("code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -d '{\"organizationId\":\"00000000-0000-0000-0000-000000000000\"}' %s%s/secrets) && [ \"$code\" = \"401\" ] || (echo \"code=$code\"; exit 1)", inClusterBaseURL, apiPath)
			code, logs := runAPIJob("api-auth-no-token-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})

		It("request with invalid token should return 400", func() {
			script := fmt.Sprintf("code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -H 'Warden-Access-Token: invalid-token' -d '{\"organizationId\":\"00000000-0000-0000-0000-000000000000\"}' %s%s/secrets) && [ \"$code\" = \"400\" ] || (echo \"code=$code\"; exit 1)", inClusterBaseURL, apiPath)
			code, logs := runAPIJob("api-auth-invalid-token-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})
	})

	Context("Secrets API", func() {
		It("GET /rest/api/1/secrets (ListSecrets) should return 200 with data array", func() {
			credPath := utils.BitwardenCredMountPath()
			script := fmt.Sprintf("TOKEN=$(cat %s/token) && ORG=$(cat %s/organization_id) && code=$(curl -k -s -o /tmp/out -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -H \"Warden-Access-Token: $TOKEN\" -d \"{\\\"organizationId\\\":\\\"$ORG\\\"}\" %s%s/secrets) && [ \"$code\" = \"200\" ] && grep -q '\"data\"' /tmp/out || (echo \"code=$code\"; cat /tmp/out; exit 1)", credPath, credPath, inClusterBaseURL, apiPath)
			code, logs := runAPIJob("api-list-secrets-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})

		It("POST /rest/api/1/secret (CreateSecret) then GET and DELETE", func() {
			credPath := utils.BitwardenCredMountPath()
			// Create, extract id, Get, Update, Delete; each step must succeed (exit 0).
			// Bitwarden Secrets Manager requires projectIds when creating a secret.
			script := fmt.Sprintf(`
TOKEN=$(cat %s/token) && ORG=$(cat %s/organization_id) && PROJECT=$(cat %s/project_id) && BASE=%s
BODY="{\"key\":\"e2e-api-crud\",\"value\":\"v1\",\"note\":\"e2e\",\"organizationId\":\"$ORG\",\"projectIds\":[\"$PROJECT\"]}"
code=$(curl -k -s -o /tmp/create.json -w '%%{http_code}' -X POST -H 'Content-Type: application/json' -H "Warden-Access-Token: $TOKEN" -d "$BODY" "$BASE%s/secret") && [ "$code" = "200" ] || (echo "create code=$code"; exit 1)
id=$(grep -o '"id":"[^"]*"' /tmp/create.json | head -1 | cut -d'"' -f4) && [ -n "$id" ] || (echo "no id"; exit 1)
code=$(curl -k -s -o /tmp/get.json -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -H "Warden-Access-Token: $TOKEN" -d "{\"id\":\"$id\"}" "$BASE%s/secret") && [ "$code" = "200" ] || (echo "get code=$code"; exit 1)
code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X PUT -H 'Content-Type: application/json' -H "Warden-Access-Token: $TOKEN" -d "{\"id\":\"$id\",\"key\":\"e2e-api-crud\",\"value\":\"v2\",\"note\":\"updated\",\"organizationId\":\"$ORG\",\"projectIds\":[\"$PROJECT\"]}" "$BASE%s/secret") && [ "$code" = "200" ] || (echo "update code=$code"; exit 1)
code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X DELETE -H 'Content-Type: application/json' -H "Warden-Access-Token: $TOKEN" -d "{\"ids\":[\"$id\"]}" "$BASE%s/secret") && [ "$code" = "200" ] || (echo "delete code=$code"; exit 1)
`, credPath, credPath, credPath, inClusterBaseURL, apiPath, apiPath, apiPath, apiPath)
			code, logs := runAPIJob("api-crud-"+utils.GetRandomString(5), strings.TrimSpace(script))
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})

		It("GET /rest/api/1/secret with non-existent ID should return 400", func() {
			credPath := utils.BitwardenCredMountPath()
			script := fmt.Sprintf("TOKEN=$(cat %s/token) && code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -H \"Warden-Access-Token: $TOKEN\" -d '{\"id\":\"00000000-0000-0000-0000-000000000000\"}' %s%s/secret) && [ \"$code\" = \"400\" ] || (echo \"code=$code\"; exit 1)", credPath, inClusterBaseURL, apiPath)
			code, logs := runAPIJob("api-get-nonexistent-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})

		It("GET /rest/api/1/secrets-by-ids with empty ids should return 200 or 400", func() {
			credPath := utils.BitwardenCredMountPath()
			script := fmt.Sprintf("TOKEN=$(cat %s/token) && code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X GET -H 'Content-Type: application/json' -H \"Warden-Access-Token: $TOKEN\" -d '{\"ids\":[]}' %s%s/secrets-by-ids) && ( [ \"$code\" = \"200\" ] || [ \"$code\" = \"400\" ] ) || (echo \"code=$code\"; exit 1)", credPath, inClusterBaseURL, apiPath)
			code, logs := runAPIJob("api-secrets-by-ids-empty-"+utils.GetRandomString(5), script)
			Expect(code).To(Equal(0), "expected exit 0: %s", logs)
		})
	})
})

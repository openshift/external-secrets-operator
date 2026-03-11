# E2E Test Suites

Run e2e tests from this repo’s root with `make test-e2e`. Use `E2E_GINKGO_LABEL_FILTER` to run a specific suite by label.

---

## Running tests

From the repo root:

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER="<label-filter>"
```

Default (if omitted): `E2E_GINKGO_LABEL_FILTER="Platform: isSubsetOf {AWS}"` (AWS-only).

### Running the whole suite

To run **all** e2e specs (every suite, regardless of label), pass an empty label filter:

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER=""
```

Ensure pre-requisites for each suite you care about are in place (see [Suites by label](#suites-by-label)). Suites whose pre-requisites are not met will skip (e.g. missing `bitwarden-creds` or `aws-creds`). To run only a subset of suites, use a label filter (see [Running multiple suites](#running-multiple-suites)).

### Failure artifacts (debugging)

When a spec fails, the suite dumps logs and cluster state into **`<output-dir>/e2e-artifacts/failure-<timestamp>/`** so you can debug later. The output directory is **`ARTIFACT_DIR`** when set (e.g. in OpenShift CI); otherwise, when running locally via `make test-e2e`, it is **`_output`** at the repo root.

Each failure dump includes:

- **`pods/`** – Last 500 lines of logs per pod and `describe` (YAML) for operator, operand, and test namespaces.
- **`events/`** – Recent events per namespace.
- **`resources/`** – ExternalSecretsConfig (cluster), ClusterSecretStores, ExternalSecrets, and PushSecrets (YAML).

JUnit and JSON reports are also written to the same output directory (see root README Testing section).

---

## Suites by label

### Platform:AWS (AWS Secret Manager)

| Item | Details |
|------|--------|
| **Label filter** | `"Platform: isSubsetOf {AWS}"` (default) |
| **Pre-requisites** | Cluster on AWS (e.g. OpenShift on AWS); K8s secret `aws-creds` in namespace `kube-system` with keys `aws_access_key_id` and `aws_secret_access_key` (credentials for AWS Secrets Manager in `ap-south-1`). On OpenShift on AWS this secret is typically made available in `kube-system` by the platform. |
| **Make command** | `make test-e2e` or `make test-e2e E2E_GINKGO_LABEL_FILTER="Platform: isSubsetOf {AWS}"` |

---

### Provider:Bitwarden (ESO CR–based Bitwarden)

When this suite runs, it **enables Bitwarden in ExternalSecretsConfig** and **creates the TLS secret** (certificate for bitwarden-sdk-server) in the operand namespace. The default e2e cluster CR does not enable Bitwarden; enabling and the TLS secret are done only when tests with the Bitwarden label run. See `config/crd/bases/operator.openshift.io_externalsecretsconfigs.yaml` for `plugins.bitwardenSecretManagerProvider` (mode, secretRef).

| Item | Details |
|------|--------|
| **Label filter** | `"Provider:Bitwarden"` |
| **Pre-requisites** | ESO installed. Create the Bitwarden credentials secret **`bitwarden-creds`** in **`external-secrets-operator`** (see [Creating the Bitwarden credentials secret](#creating-the-bitwarden-credentials-secret)). The suite enables the plugin and creates the TLS secret. Optional env: `BITWARDEN_SDK_SERVER_URL` (default: `https://bitwarden-sdk-server.external-secrets.svc.cluster.local:9998`). |
| **Make command** | `make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:Bitwarden"` |

---

### API:Bitwarden (Direct HTTP to bitwarden-sdk-server)

| Item | Details |
|------|--------|
| **Label filter** | `"API:Bitwarden"` |
| **Pre-requisites** | bitwarden-sdk-server reachable (e.g. in-cluster). For Secrets API tests: create the Bitwarden credentials secret **`bitwarden-creds`** in **`external-secrets-operator`** (see [Creating the Bitwarden credentials secret](#creating-the-bitwarden-credentials-secret)). Optional env: `BITWARDEN_SDK_SERVER_URL` (default: `https://bitwarden-sdk-server.external-secrets.svc.cluster.local:9998`). |
| **Make command** | `make test-e2e E2E_GINKGO_LABEL_FILTER="API:Bitwarden"` |

#### Creating the Bitwarden credentials secret

The Provider:Bitwarden and API:Bitwarden suites expect the secret to be named **`bitwarden-creds`** in namespace **`external-secrets-operator`**. The secret must have keys **`token`** (Bitwarden machine account access token), **`organization_id`**, and **`project_id`**.

**From literal values:**

```bash
kubectl create secret generic bitwarden-creds \
  --namespace=external-secrets-operator \
  --from-literal=token="YOUR_BITWARDEN_ACCESS_TOKEN" \
  --from-literal=organization_id="YOUR_ORGANIZATION_ID" \
  --from-literal=project_id="YOUR_PROJECT_ID"
```

**From env vars (avoids secrets in shell history):**

```bash
export BITWARDEN_ACCESS_TOKEN="..."
export BITWARDEN_ORGANIZATION_ID="..."
export BITWARDEN_PROJECT_ID="..."
kubectl create secret generic bitwarden-creds \
  --namespace=external-secrets-operator \
  --from-literal=token="${BITWARDEN_ACCESS_TOKEN}" \
  --from-literal=organization_id="${BITWARDEN_ORGANIZATION_ID}" \
  --from-literal=project_id="${BITWARDEN_PROJECT_ID}"
```

---

### CrossPlatform:GCP-AWS (GCP cluster, AWS Secrets Manager)

Cluster runs on **GCP**; the test uses a **ClusterSecretStore** backed by **AWS Secrets Manager**. You must create a Kubernetes secret that holds AWS credentials in a **fixed** name and namespace (see below).

| Item | Details |
|------|--------|
| **Label filter** | `"CrossPlatform:GCP-AWS"` |
| **Pre-requisites** | Cluster on GCP (or any non-AWS cluster). Create the AWS credentials secret with **name** `aws-creds` in **namespace** `kube-system` (see below). |
| **Make command** | `make test-e2e E2E_GINKGO_LABEL_FILTER="CrossPlatform:GCP-AWS"` |

#### Creating the AWS credentials secret

The test expects the secret to be named **`aws-creds`** in namespace **`kube-system`**. The secret must have keys **`aws_access_key_id`** and **`aws_secret_access_key`**. The test uses AWS region `ap-south-1`.

**From literal values:**

```bash
kubectl create secret generic aws-creds \
  --namespace=kube-system \
  --from-literal=aws_access_key_id="YOUR_AWS_ACCESS_KEY_ID" \
  --from-literal=aws_secret_access_key="YOUR_AWS_SECRET_ACCESS_KEY"
```

**From env vars (avoids secrets in shell history):**

```bash
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
kubectl create secret generic aws-creds \
  --namespace=kube-system \
  --from-literal=aws_access_key_id="${AWS_ACCESS_KEY_ID}" \
  --from-literal=aws_secret_access_key="${AWS_SECRET_ACCESS_KEY}"
```

---

## Running multiple suites

To run more than one label (e.g. Bitwarden provider and API):

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:Bitwarden || API:Bitwarden"
```

See [Ginkgo label documentation](https://onsi.github.io/ginkgo/#spec-labels) for label query syntax.

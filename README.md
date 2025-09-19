# external-secrets-operator for Red Hat OpenShift
This repository contains External Secrets Operator for Red Hat OpenShift. The operator runs in `external-secrets-operator` namespace.
The External Secrets Operator provides the ability to deploy [`external-secrets`](https://github.com/openshift/external-secrets) using different configurations

The External Secrets Operator for Red Hat OpenShift operates as a cluster-wide service to deploy and manage the external-secrets
application. The external-secrets application integrates with external secrets management systems and performs secret fetching,
refreshing, and provisioning within the cluster.

## Description
Use the External Secrets Operator for Red Hat OpenShift to integrate external-secrets application with the
OpenShift Container Platform cluster. The external-secrets application fetches secrets stored in the external providers such as
AWS Secrets Manager, HashiCorp Vault, Google Secrets Manager, Azure Key Vault, IBM Cloud Secrets Manager,
AWS Systems Manager Parameter Store and integrates them with Kubernetes in a secure manner.

Using the External Secrets Operator ensures the following:
- Decouples applications from the secret-lifecycle management.
- Centralizes secret storage to support compliance requirements.
- Enables secure and automated secret rotation.
- Supports multi-cloud secret sourcing with fine-grained access control.
- Centralizes and audits access control.

The External Secrets Operator for Red Hat OpenShift uses the [`external-secrets`](https://github.com/openshift/external-secrets) helm charts
to install application. The operator has three controllers to achieve the same:
- `external_secrets_manager` controller: This is responsible for
  * reconciling the `externalsecretsmanagers.openshift.operator.io` resource.
  * providing the status of other controllers.
- `external_secrets` controller: This is responsible for
  * reconciling the `externalsecretsconfig.openshift.operator.io` resource.
  * installing and managing the `external-secrets` application based on the user defined configurations in `externalsecretsconfig.openshift.operator.io` resource.
  * reconciling the `externalsecretsmanagers.openshift.operator.io` resource for the global configurations and updates the `external-scerets` deployment accordingly.
- `crd_annotator` controller:
  * This is responsible for adding `cert-manager.io/inject-ca-from` annotation in the `external-secrets` provided CRDs.
  * This is an optional controller, which will be activated only when [`cert-manager`](https://cert-manager.io/) is installed.
  * When `cert-manager` is installed after External Secrets Operator installation, `external-secrets-operator-controller-manager` deployment must be restarted to activate the controller.

The operator automatically creates a cluster-scoped `externalsecretsmanagers.openshift.operator.io` object named `cluster`.

For more information about
- `external-secrets-operator for Red Hat OpenShift`, refer to the [link](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/security_and_compliance/external-secrets-operator-for-red-hat-openshift)
- `external-secrets` application, refer to the [link](https://external-secrets.io/latest/).
- `cert-manager Operator for Red Hat OpenShift`, refer to the [link](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/security_and_compliance/cert-manager-operator-for-red-hat-openshift)

## Getting Started

### Prerequisites
- go version 1.23.6+
- docker version 17.03+.
- kubectl version v1.32.1+.
- Access to a Kubernetes v1.32.1+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/external-secrets-operator:<tag>
```

> **NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/external-secrets-operator:<tag>
```

> **NOTE:** If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE:** Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/external-secrets-operator:tag
```

> **NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/external-secrets-operator/<tag or branch>/dist/install.yaml
```

> **NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Contributing
We welcome contributions from the community! To contribute:

- Fork this repository and create a new branch.
- Make your changes and test them thoroughly.
- Run make targets to verify the behavior.
- Submit a Pull Request describing your changes and the motivation behind them.
- Run make help to view all available development targets.

We appreciate issues, bug reports, feature requests, and feedback!

## License

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


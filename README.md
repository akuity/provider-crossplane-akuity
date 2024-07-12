# provider-akuity

The `provider-akuity` repository is the Crossplane infrastucture provider for
[Akuity Platform](https://akuity.io/akuity-platform/). The provider that is built from the source code
in this repository can be installed into a Crossplane control plane and adds the 
following new functionality:

* Custom Resource Definitions (CRDs) that model Akuity services(e.g, Argo CD instances and clusters)
* Controllers to provision these resources in Akuity Cloud based on users 
desired state captured in the CRDs they create

## Installation

### Configure an API Key
The Akuity Crossplane provider needs to be configured with Akuity API credentials. Please check how to create an API key on  [Akuity Platform API Key Documentation](https://docs.akuity.io/organizations/api-keys). 

Once you have an API key and secret, create a JSON file with the following contents (replace the placeholders for API key 
ID and secret with the credentials generated above):

```
{"apiKeyId": "MY_AKUITY_API_KEY_ID", "apiKeySecret": "MY_AKUITY_API_KEY_SECRET"}
```

Next, base64 encode the above content before pasting it in the Kubernetes `Secret` in [examples/provider/config.yaml](./examples/provider/config.yaml):

```
cat myfile.json | base64
```

### Configure the Organization ID
The `ProviderConfig` resource in [examples/provider/config.yaml](./examples/provider/config.yaml) needs to be updated with your
Akuity organization ID. Your organization ID can be found by logging in to [https://akuity.cloud](https://akuity.cloud) and
navigating to your organization page. The organization ID is displayed on the top right corner of the screen.

Once you have completed the steps above to replace the placeholders for API credentials, organization ID in
[examples/provider/config.yaml](./examples/provider/config.yaml), you can install the provider by applying the `Provider` resources to your cluster:

```
kubectl apply -f examples/provider/config.yaml
```

Now you can start managing Akuity instances and clusters using Crossplane. You can view minimal examples of these resources
in:
- [Argo CD Instance](./examples/instance/basic.yaml)
- [Cluster](./examples/cluster/basic.yaml)

for detailed examples of these resources in:
- [Argo CD Instance](./examples/instance/detailed.yaml)
- [Cluster](./examples/cluster/detailed.yaml)

For all supported fields for Akuity resources, please check [https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity](https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity/).

**Note** - Managing ArgoCD secrets for instances using the Crossplane provider is not supported. The Akuity API does not currently support exporting
secret values, which makes it impossible to compare the desired and actual state of the secrets in a reconciliation loop. Please let us know if this is something
you would like to see supported by opening an issue for this repository.

## Local Development

### Crossplane

For general Crossplane getting started guides, installation, deployment, and administration, see
the Crossplane [Documentation](https://crossplane.io/docs).

### Initialize Git Submodules

Upbound provides a build submodule that is used for most of the Makefile targets. To initialize the
submodule run:

`git submodule update --init`

### Generating the Akuity resource CRDs

To generate the CRDs used by Crossplane for Akuity resources:

`make generate`

The CRDs are generated from the types defined in [apis/core/v1alpha1/](./apis/core/v1alpha1/).

### Running the provider locally

To run the Crossplane provider locally using a Kind cluster and apply the CRDs generated above:

`make dev`

The above command should only be run once. It will error if a Kind cluster already exists.

If you need to make changes to the CRDs, you can regenerate them and apply them to the already
running cluster:

```
make generate
kubectl apply -f packages/crds/
```

If you need to make changes to the provider Go code, you can Ctrl-C to quit the binary
started with `make dev` and start it again to include your changes:

`make run`

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/akuityio/provider-crossplane-akuity/issues).

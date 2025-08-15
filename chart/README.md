# Helm chart

This directory contains the _helm_ chart used to deploy the service to an Kubernetes cluster.

The chart currently supports two variants: one for _OpenShift_, intended for production environments, and another
for _Kind_, intended for development and testing environments. The default is `kind` and it can be changed setting
the `variant` value to `openshift`.

By default the log level of all the components is set to `info`. This can be changed to `error` or `debug` via
the `log.level` value.

By default the chart uses a matching version of the container image. For example, versio `0.0.9` of the chart uses
version `0.0.9` of the image. You can change that using the `images.service` value.

For example, to deploy to a _Kind_ cluster using image `quay.io/myuser/myimage:0.0.10` and setting the log level
to `debug`:

```shell
$ helm install fulfillment-service oci://ghcr.io/innabox/charts/fulfillment-service \
--namespace innabox \
--create-namespace \
--set variant=kind \
--set images.service=quay.io/myuser/myimage:0.0.10 \
--set log.level=debug \
--wait
```

In development environments it is convenient to install using the very latest source code of the chart instead
of a published release. In that case clone the repository and run the following commands to build your own
image and push it to your container registry:

```shell
my_image="quay.io/my_repo/my_image:my_tag"
podman build -t "${my_image}" .
podman push  "${my_image}"
```

Make sure to replace `quay.io/...` with your own registry, repository image and tag, and make sure to login to that
registry (with `podman login ...`) before running these commands.

Then run this to install the chart using the image that you built:

```shell
$ helm install fulfillment-service chart \
--namespace innabox \
--create-namespace \
--set variant=kind \
--set images.service=${my_image} \
--set log.level=debug \
--wait
```

For more details about the available configuration values see the comments in the `chart/values.yaml` file.

## OpenShift

Install the _cert-manager_ operator:

```shell
$ oc new-project cert-manager-operator

$ oc create -f <<.
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  namespace: cert-manager-operator
  name: cert-manager-operator
spec:
  upgradeStrategy: Default
.

$ oc create -f - <<.
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  namespace: openshift-operators
  name: cert-manager
spec:
  channel: stable
  installPlanApproval: Automatic
  name: cert-manager
  source: community-operators
  sourceNamespace: openshift-marketplace
.
```

To install the application

```shell
$ helm install fulfillment-service oci://ghcr.io/innabox/charts/fulfillment-service \
--namespace innabox \
--create-namespace \
--set variant=openshift
```

## Kind

To create the Kind cluster run a command like this:

```yaml
$ kind create cluster --config - <<.
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
name: innabox
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 8000
    listenAddress: 0.0.0.0
.
```

Install the _cert-manager_ operator:

```shell
$ helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
--version v1.18.2 \
--namespace cert-manager \
--create-namespace \
--set crds.enabled=true \
--wait
```

To install the application

```shell
$ helm install fulfillment-service oci://ghcr.io/innabox/charts/fulfillment-service \
--namespace innabox \
--create-namespace \
--set variant=kind \
--wait
```

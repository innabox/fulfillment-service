# Kubernetes deployment

This directory contains the manifests used to deploy the service to an Kubernetes cluster.

There are currently two variants of the manifests: one for OpenShift, intended for production environments, and another
for Kind, intended for development and testing environments.

## OpenShift

The gRPC protocol is based on HTTP2, which isn't enabled by default in OpenShift. To enable it run this command:

```shell
$ oc annotate ingresses.config/cluster ingress.operator.openshift.io/default-enable-http2=true
```

Install the _cert-manager_ operator:

```shell
$ oc new-project cert-manager-operator

$ oc create -f - <<.
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

Install the _Authorino_ operator:

```shell
$ oc create -f - <<.
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  namespace: openshift-operators
  name: authorino-operator
spec:
  name: authorino-operator
  sourceNamespace: openshift-marketplace
  source: redhat-operators
  channel: stable
  installPlanApproval: Automatic
.
```

To deploy the application run this:

```shell
$ oc apply -k manifests/overlays/openshift
```

## Kind

To create the Kind cluster create use a command like this:

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
    listenAddress: "0.0.0.0"
  - containerPort: 30001
    hostPort: 8001
    listenAddress: "0.0.0.0"
.
```

Install the _cert-manager_ operator:

```shell
$ kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml
```


Install the _Authorino_ operator. Due to an issue with the configuration of custom CA certificates in the Authorino
operator (see [here](https://github.com/Kuadrant/authorino-operator/pull/282)) it is necessary to replace the operator
image. So you need to download the manifests, replace the image, and then apply the result:

```shell
$ curl -o authorino.yaml https://raw.githubusercontent.com/Kuadrant/authorino-operator/refs/heads/release-v0.22.0/config/deploy/manifests.yaml
$ sed -i 's|quay.io/kuadrant/authorino-operator:v0.22.0|quay.io/innabox/authorino-operator:latest|g' authorino.yaml
$ kubectl apply -f authorino.yaml
```

Deploy the application:

```shell
$ kubectl apply -k manifests/overlays/kind
```

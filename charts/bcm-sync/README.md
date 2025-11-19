# BCM Inventory Synchronizer Helm Chart

This Helm chart deploys the BCM (Bright Cluster Manager) inventory synchronizer component for the fulfillment service.

## Prerequisites

- Kubernetes cluster (_Kind_ or _OpenShift_).
- Fulfillment service deployed and accessible via gRPC.

## Installation

To install the chart with the release name `bcm-sync`:

```bash
$ helm install bcm-sync ./charts/bcm-sync -n innabox --create-namespace
```

## Configuration

The following table lists the configurable parameters of the chart and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `bcm.url` | URL of the BCM API endpoint (e.g., https://head.internal.bcm:8081) | None (required) |
| `bcm.credentialsSecret` | Name of the secret containing BCM credentials (tls.crt and tls.key) | None (required) |
| `bcm.syncInterval` | Interval between synchronization cycles (e.g., 10s, 1m, 5m) | `10s` |
| `certs.caBundle.configMap` | Name of configmap containing trusted CA certificates in PEM format | None |
| `log.level` | Log level for all components (debug, info, warn, error) | `info` |
| `log.headers` | Enable logging of HTTP/gRPC headers | `false` |
| `log.bodies` | Enable logging of HTTP/gRPC request and response bodies | `false` |
| `images.service` | Fulfillment service container image | `ghcr.io/innabox/fulfillment-service:main` |
| `grpc.server.address` | Address of the fulfillment service gRPC server | `fulfillment-api:8000` |

### Example custom values

To customize the deployment, create a `values.yaml` file:

```yaml
bcm:
  url: https://head.internal.bcm:8081
  credentialsSecret: bcm-credentials
  syncInterval: 10s

certs:
  caBundle:
    configMap: ca-bundle

log:
  level: debug
  headers: false
  bodies: false

grpc:
  server:
    address: fulfillment-api:8000
```

### Creating the BCM Credentials Secret

Before deploying, create a Kubernetes secret with BCM credentials:

```bash
kubectl create secret tls bcm-credentials \
  --cert=admin.pem \
  --key=admin.key \
  -n innabox
```

### Installing the Synchronizer

To install the inventory synchronizer, create a `values.yaml` file with the required configuration:

```yaml
bcm:
  url: "https://head.internal.bcm:8081"
  credentialsSecret: "bcm-credentials"
  syncInterval: "10s"
```

Then install the Helm release:

```bash
helm install bcm-sync ./charts/bcm-sync \
  -f values.yaml \
  -n innabox
```

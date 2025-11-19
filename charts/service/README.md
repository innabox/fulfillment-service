# Service Helm Chart

This Helm chart deploys the complete fulfillment service.

## Prerequisites

- Kubernetes cluster (_Kind_ or _OpenShift_).
- _cert-manager_ operator installed.
- _trust-manager_ operator installed (optional, but highly recommended).
- _Authorino_ operator installed.

## Installation

To install the chart with the release name `fulfillment-service`:

```bash
$ helm install fulfillment-service ./charts/service -n innabox --create-namespace
```

## Configuration

The following table lists the configurable parameters of the chart and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `variant` | Deployment variant (`kind` or `openshift`) | `kind` |
| `certs.issuerRef.kind` | Kind of _cert-manager_ issuer | `ClusterIssuer` |
| `certs.issuerRef.name` | Name of _cert-manager_ issuer | None |
| `certs.caBundle.configMap` | Name of configmap containing trusted CA certificates in PEM format | None |
| `hostname` | Hostname used to access the service from outside the cluster | None |
| `auth.issuerUrl` | OAuth issuer URL for authentication | `https://keycloak.keycloak.svc.cluster.local:8001/realms/innabox` |
| `log.level` | Log level for all components (debug, info, warn, error) | `info` |
| `log.headers` | Enable logging of HTTP/gRPC headers | `false` |
| `log.bodies` | Enable logging of HTTP/gRPC request and response bodies | `false` |
| `images.service` | Fulfillment service container image | `ghcr.io/innabox/fulfillment-service:main` |
| `images.postgres` | PostgreSQL container image | `quay.io/sclorg/postgresql-15-c9s:latest` |
| `images.envoy` | Envoy proxy container image | `docker.io/envoyproxy/envoy:v1.33.0` |
| `database.storageSize` | Size of database persistent volume | `10Gi` |
| `dns.server` | DNS server address for dynamic updates (e.g., `my-dns-server.com:53`) | None |
| `dns.zone` | DNS zone for dynamic updates (e.g., `my.demo.`) | None |
| `dns.secret` | Name of Kubernetes secret containing TSIG key name and secret | None |

### Example custom values

To customize the deployment, create a `values.yaml` file:

```yaml
variant: openshift

certs:
  issuerRef:
    kind: Issuer
    name: my-issuer
  caBundle:
    configMap: my-ca-bundle

hostname: fulfillment-service.example.com

auth:
  issuerUrl: https://keycloak.example.com/realms/innabox

log:
  level: debug
  headers: true
  bodies: true

images:
  service: ghcr.io/innabox/fulfillment-service:v1.0.0

database:
  storageSize: 50Gi

dns:
  server: head.internal.demo:53
  zone: my.demo.
  secret: dns-tsig-secret
```

Then install with:

```bash
$ helm install fulfillment-service ./charts/service -n innabox -f values.yaml
```

## Variants

### Kind variant

When `variant: kind` is set:
- The API service uses NodePort type with port 30000
- Suitable for development and testing

### OpenShift variant

When `variant: openshift` is set:
- The API service uses ClusterIP type
- An OpenShift Route is created for external access
- Suitable for production deployments on OpenShift

## Uninstallation

To uninstall the chart:

```bash
helm uninstall fulfillment-service -n innabox
```

## Database

The chart includes a complete _PostgreSQL_ database deployment.

## DNS Configuration

The controller can perform dynamic DNS updates using RFC 2136. To enable this feature, configure the DNS settings in
your values file:

```yaml
dns:
  server: head.internal.demo:53
  zone: my.demo.
  secret: dns-tsig-secret
```

### Creating the DNS TSIG Secret

If your DNS server requires TSIG authentication, create a Kubernetes secret with the TSIG key name and secret:

```bash
kubectl create secret generic dns-tsig-secret \
  --from-literal=key="externaldns-key." \
  --from-literal=secret="q2Rg2mWmEmoMu8cu+P2rAb6gQOzrSqdQjIocOy/0u2Q=" \
  -n innabox
```

The secret must contain two keys:
- `key`: The TSIG key name
- `secret`: The TSIG key secret value

See `files/dns-secret-example.yaml` for a complete example.

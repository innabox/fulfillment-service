# Keycloak Helm chart

This Keycloak Helm chart is intended for use in the integration tests of the
fulfillment service inside a _kind_ cluster. It provides a pre-configured
Keycloak instance with the necessary realm and client configurations for testing
authentication and authorization workflows.

## Installation

To install the Keycloak chart, run the following command:

```bash
$ helm install keycloak charts/keycloak --namespace keycloak --create-namespace --wait
```

To uninstall it:

```bash
$ helm uninstall keycloak --namespace keycloak
```

## Accessing the console

The Keycloak console will be available at
`https://keycloak.keycloak.svc.cluster.local:8001`, but this address is not
resolvable via DNS from outside the cluster. For external access, the console is
available at the localhost IP address, and port 8001. To access it directly
using the DNS name add the following to your `/etc/hosts` file:

```
127.0.0.1 keycloak.keycloak.svc.cluster.local
```

The go to `https://keycloak.keycloak.svc.cluster.local:8001` from your
local machine.

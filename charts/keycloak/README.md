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

Note that this chart requires a _cert-manager_ issuer to generate the necessary
TLS certificates for _Keycloak_ and its _PostgreSQL_ database. By default, the
chart uses the `default-ca` cluster issuer, which is automatically available in
the integration tests environment.

When installing to a plain vanilla Kind cluster or any other Kubernetes cluster,
you will need to:

1. Install _cert-manager_ if not already present.

2. Create a cluster issuer.

3. Configure the chart to use your issuer by setting the `issuerRef` values:

    ```bash
    $ helm install keycloak charts/keycloak \
    --namespace keycloak \
    --create-namespace \
    --set issuerRef.name=my-issuer \
    --wait
    ```

    You can also use an issuer in the same namespace. In that case you will also
    need to change the `issuerRef.kind` value to `Issuer`:

    ```bash
    $ helm install keycloak charts/keycloak \
      --namespace keycloak \
      --create-namespace \
      --set issuerRef.kind=Issuer \
      --set issuerRef.name=my-issuer \
      --wait
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

# Exporting the realm

To export the realm configuration to a JSON file, you need to find the Keycloak
pod and execute the `export` command inside it. The exported data can be written
to a local JSON file using the following steps:

1. First, find the name of the Keycloak pod:

    ```bash
    $ pod=$(kubectl get pods -n keycloak -l app=keycloak-service -o json | jq -r '.items[].metadata.name')
    ```

2. Run the `export` command inside the pod to write the ream to a temporary file:

    ```bash
    $ kubectl exec -n keycloak "${pod}" -- /opt/keycloak/bin/kc.sh export --realm innabox --file /tmp/realm.json
    ```

3. Copy the temporary file to a local file:

    ```bash
    $ kubectl exec -n keycloak "${pod}" -- cat /tmp/realm.json > realm.json
    ```

4. Optionally, if you want to replace the realm used by the chart, overwrite the
   `realm.json` file:

   ```bash
   $ cp realm.json charts/keycloak/files/ream.json
   ```

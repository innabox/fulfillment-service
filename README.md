# Fulfillment service

This project contains the code for the fulfillment service.

## Required development tools

To work with this project you will need the following tools:

- [Go](https://go.dev) - Used to build the Go code.
- [Buf](https://buf.build) - Used to generate Go code from gRPC specifications.
- [Ginkgo](https://onsi.github.io/ginkgo) - Used to run unit tests.
- [Kustomize](https://kustomize.io) - Used to generate Kubernetes manifests.
- [Kubectl](https://kubernetes.io/es/docs/reference/kubectl) - Used to deploy to an OpenShift cluster.
- [PostgreSQL](https://www.postgresql.org) - Used to store persistent state.
- [Podman](https://podman.io) - Used to build and run container images.
- [gRPCurl](https://github.com/fullstorydev/grpcurl) - Used to test the gRPC API from the CLI.
- [curl](https://curl.se) - Used to test the REST API from the CLI.
- [jq](https://jqlang.org) - Used by some of the commands in this document.

## Building the binary

To build the `fulfillment-service` binary run `go build`.

## Running unit tests

To run unit the unit tests run `ginkgo run -r`.

## Running PostgreSQL

To quickly run a local postgresql database in a container, run the following command:

```
podman run -d --name postgresql_database \
  -e POSTGRESQL_USER=user -e POSTGRESQL_PASSWORD=pass -e POSTGRESQL_DATABASE=db \
  -p 127.0.0.1:5432:5432 quay.io/sclorg/postgresql-15-c9s:latest
```

Done!

Or if you prefer to install and run postgresql directly on your development
system, you'll need to create a database for the service. For example, assuming
that you already have administrator access to the database, you can create a
user `user` with password `pass` and a database `db` with the following
commands:

    postgres=# create user user with password 'pass';
    CREATE ROLE
    postgres=# create database db owner user;
    CREATE DATABASE
    postgres=#

## Running the fulfillment-service

To run the the gRPC server use a command like this:

    $ ./fulfillment-service  start server \
    --log-level=debug \
    --log-headers=true \
    --log-bodies=true \
    --grpc-listener-address=localhost:8000 \
    --db-url=postgres://user:pass@localhost:5432/db

To run the the REST gateway use a command like this:

    $ ./fulfillment-service start gateway \
    --log-level=debug \
    --log-headers=true \
    --log-bodies=true \
    --http-listener-address=localhost:8001 \
    --grpc-server-address=localhost:8000 \
    --grpc-server-plaintext

You may need to adjust the commands to use your database details.

To verify that the gRPC server is working use `grpcurl`. For example, to list the available gRPC services:

    $ grpcurl -plaintext localhost:8000 list
    fulfillment.v1.ClusterOrders
    fulfillment.v1.ClusterTemplates
    fulfillment.v1.Clusters
    fulfillment.v1.Events
    grpc.reflection.v1.ServerReflection
    grpc.reflection.v1alpha.ServerReflection

To list the methods available in a service, for example in the `ClusterTemplates` service:

    $ grpcurl -plaintext localhost:8000 list fulfillment.v1.ClusterTemplates
    fulfillment.v1.ClusterTemplates.Get
    fulfillment.v1.ClusterTemplates.List

To invoke a method, for example the `List` method of the `ClusterTemplates` service:

    $ grpcurl -plaintext localhost:8000 fulfillment.v1.ClusterTemplates/List
    {
      "size": 2,
      "total": 2,
      "items": [
        {
          "id": "045cbf50-a04f-4b9a-9ea5-722fd7655a24",
          "title": "my_template",
          "description": "My template is *nice*."
        },
        {
          "id": "2cf86b60-9047-45af-8e5a-efa6f92d34ae",
          "title": "your_template",
          "description": "Your template is _ugly_."
        }
      ]
    }

To verify that the REST gateway is working use `curl`. For example, to get the list of templates:

    $ curl --silent http://localhost:8001/api/fulfillment/v1/cluster_templates | jq
    {
      "size": 2,
      "total": 2,
      "items": [
        {
          "id": "045cbf50-a04f-4b9a-9ea5-722fd7655a24",
          "title": "my_template",
          "description": "My template is *nice*."
        },
        {
          "id": "2cf86b60-9047-45af-8e5a-efa6f92d34ae",
          "title": "your_template",
          "description": "Your template is _ugly_."
        }
      ]
}

## Building the container image

Select your image name, for example `quay.io/myuser/fulfillment-service:latest`, then build and tag the image with a
command like this:

    $ podman build -t quay.io/myuser/fulfillment-service:latest .

If you want to deploy to an OpenShift cluster then you will also need to push the image, so that the cluster can pull
it:

    $ podman push quay.io/myuser/fulfillment-service:latest

## Deploying to an OpenShift cluster

In order to be able to use gRPC in an OpenShift cluster it is necessary to enable HTTP/2:

    $ kubectl annotate ingresses.config/cluster ingress.operator.openshift.io/default-enable-http2=true

To deploy using the default image run the following command:

    $ kubectl apply -k manifests

To undeploy:

    $ kubectl delete -k manifests

If you want to deploy using your own image, then you will need first to edit the manifests:

    $ cd manifests
    $ kustomize edit set image fulfillment-service=quay.io/myuser/fulfillment-service:latest

The server deployed to the OpenShift cluster requires authentication. Before using it you will need to obtain the
token of the `client` service account that is created for that. Use a command like this to obtain the token and save it
into the `token` environment variable:

    $ export token=$(kubectl create token -n innabox client)

To verify that the deployment is working get the URL of the route, and use `grpcurl` and `curl` to verify that both the
gRPC server and the REST gateway are working:

    $ kubectl get route -n innabox fulfillment-api -o json | jq -r '.spec.host'
    fulfillment-api-innabox.apps.mycluster.com

    $ grpcurl -insecure
    -H "Authorization: Bearer ${token}" \
    fulfillment-api-innabox.apps.mycluster.com:443 fulfillment.v1.ClusterTemplates/List
    {
      "size": 2,
      "total": 2,
      "items": [
        {
          "id": "045cbf50-a04f-4b9a-9ea5-722fd7655a24",
          "title": "my_template",
          "description": "My template is *nice*."
        },
        {
          "id": "2cf86b60-9047-45af-8e5a-efa6f92d34ae",
          "title": "your_template",
          "description": "Your template is _ugly_."
        }
      ]
    }

    $  curl --silent --insecure \
    --header "Authorization: Bearer ${token}" \
    https://fulfillment-api-innabox.apps.mycluster.com:443/api/fulfillment/v1/cluster_templates | jq
    {
      "size": 2,
      "total": 2,
      "items": [
        {
          "id": "045cbf50-a04f-4b9a-9ea5-722fd7655a24",
          "title": "my_template",
          "description": "My template is *nice*."
        },
        {
          "id": "2cf86b60-9047-45af-8e5a-efa6f92d34ae",
          "title": "your_template",
          "description": "Your template is _ugly_."
        }
      ]
    }

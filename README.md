# Quartermaster

Quartermaster sits in your Kubernetes cluster and watches resources you care about, then tells you when they are created, changed or deleted.

You tell Quartermaster via a configmap which resources you care about, e.g. namespaces and deployments.  Quartermaster watches those resources and, when an event occurs, it sends a POST request with a JSON payload to a REST API endpoint that you give it.  You can also provide more than one API endpoint and only report on specific namespaces to particular API endpoints.

## Install

Build and push a docker image:

    $ export PREFIX=[your image repo]  # e.g. quay.io/myrepo/quartermaster
    $ export TAG=[version no.]  # e.g. 0.14
    $ make release

Edit example manifests:

1. Add your image name to `examples/quartermaster-deploy.yaml`.
2. Configure as needed `examples/quartermaster-config.yaml`.  See Configuration section below.
3. If using basic auth at the remote endpoint/s that Quartermaster is reporting to, add username and password credentials to `examples/quartermaster-secrets.yaml`.  Refer to [Kubernetes secret docs](https://kubernetes.io/docs/concepts/configuration/secret/#creating-a-secret-manually) for instructions.  If not using basic auth, remove the `env` definitions in `examples/quartermaster-deploy.yaml`

Deploy Quartermaster:

    $ kubectl apply -f examples/quartermaster-rbac.yaml
    $ kubectl apply -f examples/quartermaster-config.yaml
    $ kubectl apply -f examples/quartermaster-deploy.yaml

## Testing

If you would like to see the payload that is being sent by Quartermaster for testing and development purposes:

1. Deploy the echo pod:

        $ kubectl apply -f examples/echo.yaml

2. Configure one of the `remoteEndpoints` in `examples/quartermaster-config.yaml` to use `url: http://echo/quartermaster`.
3. View the echo logs:

        $ kubectl logs -n heptio-qm echo

## Configuration

Configuration for Quartermaster is done via a config file, which, when deployed to kubernetes should take the form of a configmap. An example can be seen in the [examples](examples) dir.

### Config file options

* `clusterName` Name of the cluster Quartermaster is deployed to.  This is an arbitrary value but should be meaninfgul to the endpoint that is processing the payload, especially when it is receiving reports from multiple clusters.
* `emitCacheDuration` Amount of time quatermaster should remember it previously emitted an object and that object's state at the given time. Specified with seconds (s), minutes (m), or hours (h).
* `emitBatchMaxObjects` Maximum number of objects to be sent in a single request.  If not defined, default is 10.
* `emitInterval` Amount of time in seconds that the emissions processor sleeps between sending batches of objects.  When emissions are processed, the number of objects sent is limited by `emitBatchMaxObjects`.  Be aware that if you set `emitBatchMaxObjects` to low and the `emitInterval` too long, it may delay the reporting of changes to resources.  If not defined, default is 1s.
* `forceReuploadDuration` Amount of time quartermaster should wait before attempting to re-process all objects inside of kubernetes. If state of object during re-upload hasn't changed since last emit (meaning quartermaster still has a cached record, the object will be dropped). This setting relates to client-go's resyncPeriod Setting this value to 0 ensures no forced re-upload occurs. Specified with seconds (s), minutes (m), or hours (h).
* `prometheusMetrics` Configuration for exposing prometheus metrics:
    - `port` The port to expose the metrics on.  This value must be supplied to activate prometheus metrics.  Supply a string or int with a valid port number.
    - `path` The path at which to expose metrics.  Supply a string including the leading forward slash.  If not defined will default to "/metrics".
* `httpLiveness` Configuration for HTTP liveness check server:
    - `port` The port that the liveness check server should listen on.  This value must be supplied to start the liveness server.  Supply a string or int with a valid port number.
    - `path` The path the liveness checker will send requests to.  If not defined will default to "/live".
* `remoteEndpoints` An array of destinations to send payloads to. The following can be defined for each endpoint:
    - `type` The type of endpoint.  The following are supported:
        * `http` An HTTP or HTTPS REST API endpoint.
        * `sqs` An AWS [Simple Queue Service](https://aws.amazon.com/sqs/) endpoint.
    - `url` A standard URL that Quartermaster can reach, e.g. https://myendpoint/quartermaster
    - `usernameVar` A basic auth username used to authenticate at the remote endpoint. If you use this option you must also define an `env` with the same `name` in your deployment manifest, perferably referenced from a secret.
    - `passwordVar` A basic auth password used to authenticate at the remote endpoint. If you use this option you must also define an `env` with the same `name` in your deployment manifest, perferably referenced from a secret.
    - `namespaces` An array of namespaces to report on.  If specified, only resources in the configured namespaces will be included in the payload.  If this option is not configured, all resources from all namespaces will be reported.
* `metadata` Arbitrary key-value pairs that will be added to the root of every payload.
* `resources` An array of Kubernetes resources that Quartermaster should watch.  The following can be defined for each resource:
    - `name` The name of the resource.  The following resources are supported:
        * Kubernetes:
            - `nodes`
            - `namespaces`
            - `pods`
            - `services`
            - `ingresses`
            - `deployments`
            - `replicasets`
            - `configmaps`
            - `secrets`
        * Istio:
            - `virtualservices`
    - `assetId` An arbitrary string that can be used by the remote endpoint to help differentiate resources.
    - `pruneFields` Fields to remove from payload. Used to remove useless or sensitive data that you don't wish to send.
* `deltaUpdates` Do you wish to send a full object of all the kubernetes resources we are watching, or just the the items have have changed? This is a bool and expects either `true` or `false`.
* `ignoreNamespaces` An array of namespaces you wish to always ignore.  No events in these namespaces will ever be reported on, even if they are added to the `remoteEndpoints.namespaces`.

### CLI Parameters

The following are the available CLI parameters
```
  -c string
    	Path to quartermaster config file (default "/etc/quartermaster/config.yaml")
  -kubeconfig string
    	Path to a kubeconfig file
```

Because Quartermaster leverages the kubernetes go-client package the following parameters are also available, however they are not specific to quartermaster itself.

```
  -alsologtostderr
    	log to standard error as well as files
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
```

## Developing

A `Makefile` is included to update dependencies, build, test and create a docker image

```
deps                          Install/Update depdendencies
help                          This help info
image                         Build Docker image
release                       Build binary, build docker image, push docker image, clean up
run                           Build Docker image
test                          Run tests
```

Dependencies are managed via [dep](https://github.com/golang/dep) and will need to installed if you choose to update the dependencies. All dependencies are commited in the `vendor` dir.

To develop and test Quartermaster locally, the controller accepts a kubeconfig file to interact with a cluster. Otherwise the pod's service account would be used, which is desired for real deployment. To run against a local kubeconfig, the command would be as follows.

```bash
make run KUBECONFIG=~/.kube/config
```


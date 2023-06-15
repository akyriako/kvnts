# kvnts
A Kubernetes custom Controller to watch & forward Kubernetes Events to Grafana Loki with Promtail, push them and their correlated Logs as alerts to Slack

## Description
Refer to this [medium article](https://betterprogramming.pub/kubernetes-observability-part-1-events-logs-integration-with-slack-openai-and-grafana-62068cf43ec) for more analytical information 

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) or [K3S/K3D](https://k3d.io/) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
0. Provide enviroment variables

You are going to need to provide the values of `CLUSTER_NAME`, `LOKI_PUSH_GATEWAY_URL` and `OPENAI_API_KEY` as environment variables (just with source from an .env file if you are running locally) or as a `ConfigMap` if you are installing on a cluster 

1. Install Instances of Custom Resources:

Fill in samples with the necessary Slack tokens:

```yaml
apiVersion: events.kvnts/v1alpha1
kind: SinksConfig
metadata:
  labels:
    app.kubernetes.io/name: sinksconfig
    app.kubernetes.io/instance: sinksconfig-sample
    app.kubernetes.io/part-of: kvnts
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: kvnts
  name: sinksconfig-sample
spec:
  slack:
    botToken: "xoxb-"
    channelID: "C0"
    appLevelToken: "xapp-1-"
    debug: false
  excludedReasons: [ "FailedMount" ]
```

You can completely omit `excludedReasons`, this is just an example of how you could in a declarative way to ignore Kubernetes Events for specific set of `Reasons` 

and then install them in the cluster:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/kvnts:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/kvnts:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


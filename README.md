# loopNuke
Prevent expensive mistakes while developing event driven architecture with loopNuke!

## About
`loopNuke` is a small service that runs in the namespace you are working in. Once configured, `loopNuke` can be deployed and will begin to monitor events in the namespace. If the event threshold is met, `loopNuke` will destory the **entire** namespace.


## Installation
With `Koby` installed in the target cluster, you can register the `LoopNuke` kind by running the following command:
```
make apply
```

Now you can deploy a `LoopNuke` instance by applying the manifest located at `/config/koby/101-instance.yaml`
```
kubectl -n <namespace> apply -f /config/koby/101-instance.yaml
```

Without `Koby` one could build and deploy the `LoopNuke` instance with `ko` by running the following command:
```
ko -n <namespace> apply -f config/
```


## Local Development

### Run `loopNuke` Locally
In `dev` mode `loopNuke` will point to the namespace `test` so ensure that namespace exists prior to deploying.
```
export CLUSTER_NAME=
export DEV=true
go run cmd/loopnuke/main.go
```

### Build `loopNuke` Container

#### Create a release manifest
This adapter is built with `ko`.
use
```
make release
```
to create a file `release.yaml` in the root directory that can be deployed without `Koby` installed.

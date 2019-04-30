# Kubernetes Custom Resource Conversion Webhook Example

This is an example of setting up Kubernetes CR (Custom Resource) conversion
webhook in a GCE cluster.

This repo also contains benchmark testing to measure CR performance compared to
native Kubernetes resources.

## Set up a CRD with webhook conversion

### TL;DR

Run `setup.sh` will call the following steps and set up the cluster for you.

### Steps

1. Create a GCE cluster (n1-standard-1) with CustomResourceWebhookConversion feature enabled

```sh
KUBE_FEATURE_GATES="ExperimentalCriticalPodAnnotation=true,CustomResourceWebhookConversion=true" KUBE_UP_AUTOMATIC_CLEANUP=true $GOPATH/src/k8s.io/kubernetes/cluster/kube-up.sh
```

2. Create a secret containing a TLS key and certificate

```sh
hack/webhook-create-signed-cert.sh \
    --service webhook-service.default.svc \
    --secret webhook-tls-certs \
    --namespace default
```

3. Create a CRD with the caBundle correctly configured from the TLS certificate

```sh
cat artifacts/crd-template.yaml | hack/webhook-patch-ca-bundle.sh --secret webhook-tls-certs > artifacts/crd-with-webhook.yaml
kubectl create -f artifacts/crd-with-webhook.yaml
```

4. Create a conversion webhook that uses the TLS certificate and key

```sh
kubectl create -f artifacts/webhook-pod.yaml
kubectl create -f artifacts/webhook-service.yaml
# Wait a few seconds for endpoints to be available for service
```

5. Create custom resources at both supported versions for the CRD

```sh
kubectl create -f artifacts/foov1.yaml
kubectl create -f artifacts/foov2.yaml
```

6. Read using both versions

```sh
kubectl get foo foov1
kubectl get foo.v1.stable.example.com foov1
```

7. Create CRD without conversion

```sh
kubectl create -f artifacts/bar-crd.yaml
```

## Benchmark testing

We suggest running the benchmarks on master VM to reduce the network noise.

```sh
# Push test binary to GCE master VM
make push_test

# Move the binary to /run to execute it
sudo mv /tmp/conversion-webhook-example.test /run

# Run all benchmarks
/run/conversion-webhook-example.test -test.bench=.

# Run benchmarks for create latency
/run/conversion-webhook-example.test -test.bench=CreateLatency

# Run benchmarks for create throughput
/run/conversion-webhook-example.test -test.bench=CreateThroughput

# Run benchmarks for list
/run/conversion-webhook-example.test -test.bench=List
```

## References

- https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/
- https://github.com/morvencao/kube-mutating-webhook-tutorial/

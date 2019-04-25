1. Enable webhook conversion

If using kubernetes 1.14 or earlier, modify the kube-apiserver "--feature-gates" flag to contain "CustomResourceWebhookConversion=true".

2. Create a secret containing a TLS key and certificate

```sh
scripts/webhook-create-signed-cert.sh \
    --service webhook-service.default.svc \
    --secret webhook-tls-certs \
    --namespace default
```

3. Create a CRD with the caBundle correctly configured from the TLS certificate

```sh
cat crd-template.yaml | scripts/webhook-patch-ca-bundle.sh --secret webhook-tls-certs > crd-with-webhook.yaml
kubectl create -f crd-with-webhook.yaml
```

4. Create a conversion webhook that uses the TLS certificate and key

```sh
kubectl create -f webhook-pod.yaml
kubectl create -f webhook-service.yaml
```

5. Create custom resources at both supported versions for the CRD

```sh
kubectl create -f foov1.yaml
kubectl create -f foov2.yaml
```

6. Read using both versions

```sh
kubectl get foo foov1
kubectl get foo.v1.stable.example.com foov1
```

## References

- https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/
- https://github.com/morvencao/kube-mutating-webhook-tutorial/


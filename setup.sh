#!/bin/bash

# NOTE: this is a direct copy of README.md and meant to be up-to-dated
# accordingly

# 1. Create a GCE cluster with CustomResourceWebhookConversion feature enabled

KUBE_FEATURE_GATES="ExperimentalCriticalPodAnnotation=true,CustomResourceWebhookConversion=true" KUBE_UP_AUTOMATIC_CLEANUP=true $GOPATH/src/k8s.io/kubernetes/cluster/kube-up.sh

# 2. Create a secret containing a TLS key and certificate

hack/webhook-create-signed-cert.sh \
    --service webhook-service.default.svc \
    --secret webhook-tls-certs \
    --namespace default

# 3. Create a CRD with the caBundle correctly configured from the TLS certificate

cat artifacts/crd-template.yaml | hack/webhook-patch-ca-bundle.sh --secret webhook-tls-certs > artifacts/crd-with-webhook.yaml
kubectl create -f artifacts/crd-with-webhook.yaml

# 4. Create a conversion webhook that uses the TLS certificate and key

kubectl create -f artifacts/webhook-pod.yaml
kubectl create -f artifacts/webhook-service.yaml

# Wait a few seconds for endpoints to be available for service
sleep 10

# 5. Create custom resources at both supported versions for the CRD

kubectl create -f artifacts/foov1.yaml
kubectl create -f artifacts/foov2.yaml

# 6. Read using both versions

kubectl get foo foov1
kubectl get foo.v1.stable.example.com foov1

# 7. Create CRD without conversion

kubectl create -f artifacts/bar-crd.yaml

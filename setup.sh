#!/bin/bash

# NOTE: this is a direct copy of README.md and meant to be up-to-dated
# accordingly

# 1. Create a GCE cluster with CustomResourceWebhookConversion feature enabled

MASTER_SIZE=n1-standard-8 KUBE_FEATURE_GATES="ExperimentalCriticalPodAnnotation=true,CustomResourceWebhookConversion=true" KUBE_UP_AUTOMATIC_CLEANUP=true KUBE_APISERVER_REQUEST_TIMEOUT_SEC=600 ENABLE_APISERVER_INSECURE_PORT=true $GOPATH/src/k8s.io/kubernetes/cluster/kube-up.sh

# 2. Create a secret containing a TLS key and certificate

hack/webhook-create-signed-cert.sh \
    --service webhook-service.default.svc \
    --secret webhook-tls-certs \
    --namespace default

# 3. Create a CRD with the caBundle correctly configured from the TLS certificate

cat artifacts/crd-template.yaml | hack/webhook-patch-ca-bundle.sh --secret webhook-tls-certs > artifacts/crd-with-webhook.yaml
kubectl apply -f artifacts/crd-with-webhook.yaml

# 4. Create a conversion webhook that uses the TLS certificate and key

kubectl apply -f artifacts/webhook-pod.yaml
kubectl apply -f artifacts/webhook-service.yaml

# Wait a few seconds for endpoints to be available for service
sleep 10

# 5. Create custom resources at both supported versions for the CRD

kubectl apply -f artifacts/foov1.yaml
kubectl apply -f artifacts/foov2.yaml

# 6. Read using both versions

kubectl get foo foov1
kubectl get foo.v1.stable.example.com foov1

# 7. Create CRD without conversion

kubectl apply -f artifacts/bar-crd.yaml

# 8. Create test namespaces

kubectl create ns empty
kubectl create ns large-data
kubectl create ns large-metadata

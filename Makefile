build_config:
	@hack/patch-kube-config.sh

push_config: build_config
	@gcloud compute scp ./artifacts/kubeconfig.yaml kubernetes-master:/tmp/kubeconfig
	@echo Copied kube config to your cluster. Please run \"mkdir -p \~/.kube \&\& mv /tmp/kubeconfig \~/.kube/config\"

build_test:
	@go test -c
	@go build

push_test: push_config build_test
	@gcloud compute scp ./conversion-webhook-example.test kubernetes-master:/tmp
	@echo Copied conversion-webhook-example.test to your cluster. Please run \"sudo mv /tmp/conversion-webhook-example.test /run\"
	@gcloud compute scp ./conversion-webhook-example kubernetes-master:/tmp
	@echo Copied conversion-webhook-example to your cluster. Please run \"sudo mv /tmp/conversion-webhook-example /run\"

clean:
	@rm -f artifacts/kubeconfig.yaml

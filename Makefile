build_test:
	go test -c

push_test: build_test
	gcloud compute scp ./conversion-webhook-example.test kubernetes-master:/tmp
	@echo Copied conversion-webhook-example.test to your cluster. Please run \"sudo mv /tmp/conversion-webhook-example.test /run\"

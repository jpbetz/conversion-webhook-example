package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	err = createv1(client)
	if err != nil {
		panic(err)
	}
}

func createv1(client dynamic.Interface) error {
	fooGvr := schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	foov1Client := client.Resource(fooGvr).Namespace("default")
	count := 100
	start := time.Now()
	for i := 0; i < count; i++ {
		foo := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "stable.example.com/v1",
				"kind":       "Foo",
				"metadata": map[string]interface{}{
					"name": fmt.Sprintf("foov1-%d", i),
				},
			},
		}
		_, err := foov1Client.Create(foo, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create foo: %v", err)
		}
	}
	duration := time.Now().Sub(start)
	fmt.Printf("created %d foo v1 resources\n", count)
	fmt.Printf("duration: %v\n", duration)
	fmt.Printf("writes/s: %v\n", float64(count)/duration.Seconds())
	return nil
}

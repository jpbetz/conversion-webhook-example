package main

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func mustNewClient() dynamic.Interface {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfig = ""
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	config.QPS = 1000.0
	config.Burst = 2000
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return client
}

func mustNewClientset() *kubernetes.Clientset {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfig = ""
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	config.QPS = 1000.0
	config.Burst = 2000
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return client
}

var (
	fooGvr       = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	barGvr       = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "bars"}
	endpointsGvr = schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}
)

// BenchmarkCreateWithConvert tests for latency, not throughput.
func xBenchmarkCreateWithConvert_Latency(b *testing.B) {
	client := mustNewClient()
	foov1Client := client.Resource(fooGvr).Namespace("default")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createFoo(foov1Client, b)
	}
}

// BenchmarkCreate tests for latency, not throughput.
func xBenchmarkCreate_Latency(b *testing.B) {
	// TODO: parallelize create requests, this is doing everything in series and measures only latency usefully.
	client := mustNewClient()
	barv1Client := client.Resource(barGvr).Namespace("default")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createBar(barv1Client, b)
	}
}

// BenchmarkCreateEndpoints_Latency tests for latency, not throughput.
func xBenchmarkCreateEndpoints_Latency(b *testing.B) {
	// TODO: parallelize create requests, this is doing everything in series and measures only latency usefully.
	clientset := mustNewClientset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createEndpoints(clientset, b)
	}
}

// BenchmarkDynamicCreateEndpoints_Latency tests for latency, not throughput.
func xBenchmarkDynamicCreateEndpoints_Latency(b *testing.B) {
	// TODO: parallelize create requests, this is doing everything in series and measures only latency usefully.
	client := mustNewClient()
	endpointsClient := client.Resource(endpointsGvr).Namespace("default")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dynamicCreateEndpoints(endpointsClient, b)
	}
}

func xBenchmarkCreateWithConvert_Throughput(b *testing.B) {
	client := mustNewClient()
	foov1Client := client.Resource(fooGvr).Namespace("throughput")
	b.ResetTimer()
	var wg sync.WaitGroup
	count := 100
	start := time.Now()
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			createFoo(foov1Client, b)
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Printf("created %d CRs in %v\n", count, time.Now().Sub(start))
}

func xBenchmarkCreate_Throughput(b *testing.B) {
	client := mustNewClient()
	barv1Client := client.Resource(barGvr).Namespace("throughput")
	b.ResetTimer()
	var wg sync.WaitGroup
	count := 100
	start := time.Now()
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			createBar(barv1Client, b)
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Printf("created %d CRs in %v\n", count, time.Now().Sub(start))
}

func xBenchmarkCreateEndpoints_Throughput(b *testing.B) {
	clientset := mustNewClientset()
	b.ResetTimer()
	var wg sync.WaitGroup
	count := 100
	start := time.Now()
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			createEndpoints(clientset, b)
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Printf("created %d Endpoints in %v\n", count, time.Now().Sub(start))
}

func xBenchmarkListWithConvert(b *testing.B) {
	client := mustNewClient()
	foov1Client := client.Resource(fooGvr).Namespace("default")
	listSize := 10000
	l, err := foov1Client.List(metav1.ListOptions{})
	if err != nil {
		b.Fatalf("failed to check list size: %v", err)
	}
	if len(l.Items) < listSize {
		var wg sync.WaitGroup
		remaining := listSize - len(l.Items)
		wg.Add(remaining)
		for i := 0; i < remaining; i++ {
			go func() {
				createFoo(foov1Client, b)
				wg.Done()
			}()
		}
		wg.Wait()
	} else if len(l.Items) > listSize {
		b.Fatalf("Too many items already exist. Want %d got %d", listSize, len(l.Items))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := foov1Client.List(metav1.ListOptions{})
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func xBenchmarkList(b *testing.B) {
	client := mustNewClient()
	barv1Client := client.Resource(barGvr).Namespace("default")
	listSize := 10000
	l, err := barv1Client.List(metav1.ListOptions{})
	if err != nil {
		b.Fatalf("failed to check list size: %v", err)
	}
	if len(l.Items) < listSize {
		var wg sync.WaitGroup
		remaining := listSize - len(l.Items)
		wg.Add(remaining)
		for i := 0; i < remaining; i++ {
			go func() {
				createBar(barv1Client, b)
				wg.Done()
			}()
		}
		wg.Wait()
	} else if len(l.Items) > listSize {
		b.Fatalf("Too many items already exist. Want %d got %d", listSize, len(l.Items))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := barv1Client.List(metav1.ListOptions{})
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func xBenchmarkListEndpoints(b *testing.B) {
	clientset := mustNewClientset()
	listSize := 10000
	l, err := clientset.CoreV1().Endpoints("default").List(metav1.ListOptions{})
	if err != nil {
		b.Fatalf("failed to check list size: %v", err)
	}
	if len(l.Items) < listSize {
		var wg sync.WaitGroup
		remaining := listSize - len(l.Items)
		wg.Add(remaining)
		for i := 0; i < remaining; i++ {
			go func() {
				createEndpoints(clientset, b)
				wg.Done()
			}()
		}
		wg.Wait()
	} else if len(l.Items) > listSize {
		b.Fatalf("Too many items already exist. Want %d got %d", listSize, len(l.Items))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := clientset.CoreV1().Endpoints("default").List(metav1.ListOptions{})
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func BenchmarkDynamicListEndpoints(b *testing.B) {
	client := mustNewClient()
	endpointsClient := client.Resource(endpointsGvr).Namespace("default")
	listSize := 10000
	l, err := endpointsClient.List(metav1.ListOptions{})
	if err != nil {
		b.Fatalf("failed to check list size: %v", err)
	}
	if len(l.Items) < listSize {
		var wg sync.WaitGroup
		remaining := listSize - len(l.Items)
		wg.Add(remaining)
		for i := 0; i < remaining; i++ {
			go func() {
				dynamicCreateEndpoints(endpointsClient, b)
				wg.Done()
			}()
		}
		wg.Wait()
	} else if len(l.Items) > listSize {
		b.Fatalf("Too many items already exist. Want %d got %d", listSize, len(l.Items))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := endpointsClient.List(metav1.ListOptions{})
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func createFoo(foov1Client dynamic.ResourceInterface, b *testing.B) {
	foo := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stable.example.com/v1",
			"kind":       "Foo",
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("foov1-%d", time.Now().Nanosecond()),
			},
		},
	}
	foov1Client.Create(foo, metav1.CreateOptions{})
}

func createBar(barv1Client dynamic.ResourceInterface, b *testing.B) {
	bar := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stable.example.com/v1",
			"kind":       "Bar",
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("barv1-%d", time.Now().Nanosecond()),
			},
		},
	}
	barv1Client.Create(bar, metav1.CreateOptions{})
}

func createEndpoints(clientset *kubernetes.Clientset, b *testing.B) {
	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("endpoints-%d", time.Now().Nanosecond()),
		},
	}
	clientset.CoreV1().Endpoints("default").Create(endpoints)
}

func dynamicCreateEndpoints(client dynamic.ResourceInterface, b *testing.B) {
	endpoints := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Endpoints",
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("endpoints-%d", time.Now().Nanosecond()),
			},
		},
	}
	client.Create(endpoints, metav1.CreateOptions{})
}

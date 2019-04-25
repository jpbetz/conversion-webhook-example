package main

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return client
}

var (
	fooGvr = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	barGvr = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "bars"}
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

func BenchmarkCreateWithConvert_Throughput(b *testing.B) {
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

func xBenchmarkListWithConvert(b *testing.B) {
	client := mustNewClient()
	foov1Client := client.Resource(fooGvr).Namespace("default")
	listSize := 10000
	l, err := foov1Client.List(metav1.ListOptions{ResourceVersion: "0"})
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
	l, err := barv1Client.List(metav1.ListOptions{ResourceVersion: "0"})
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

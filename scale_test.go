package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GVR used for building dynamic client
	foov1GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	foov2GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v2", Resource: "foos"}
	barGVR       = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "bars"}
	endpointsGVR = schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}

	// TODO: make sure namespace exists in setup / test
	testNamespace = "benchmark"
)

var foov1Template = []byte(`apiVersion: stable.example.com/v1
kind: Foo
metadata:
  name: template`)

var barTemplate = []byte(`apiVersion: stable.example.com/v1
kind: Bar
metadata:
  name: template`)

var endpointsTemplate = []byte(`apiVersion: v1
kind: Endpoints
metadata:
  name: template`)

func benchmarkCreateLatency(b *testing.B, client BenchmarkClient) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Create(0)
		if err != nil {
			b.Fatalf("failed to create object: %v", err)
		}
	}
}

func BenchmarkCreateLatencyCRWithConvert(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, testNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, testNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func benchmarkCreateThroughput(b *testing.B, client BenchmarkClient) {
	b.ResetTimer()
	var wg sync.WaitGroup
	count := 100
	start := time.Now()
	wg.Add(count)
	for i := 0; i < count; i++ {
		// deep copy i
		idx := i
		go func() {
			_, err := client.Create(idx)
			if err != nil {
				b.Fatalf("failed to create object: %v", err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Printf("created %d objects in %v\n", count, time.Now().Sub(start))
}

func BenchmarkCreateThroughputCRWithConvert(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, testNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, testNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func benchmarkList(b *testing.B, client BenchmarkClient) {
	listSize := 10000
	num, err := client.Count()
	if err != nil {
		b.Fatalf("failed to check list size: %v", err)
	}
	if num < listSize {
		var wg sync.WaitGroup
		remaining := listSize - num
		wg.Add(remaining)
		for i := 0; i < remaining; i++ {
			// deep copy i
			idx := i
			go func() {
				_, err := client.Create(idx)
				if err != nil {
					b.Fatalf("failed to create object: %v", err)
				}
				wg.Done()
			}()
		}
		wg.Wait()
	} else if num > listSize {
		b.Fatalf("Too many items already exist. Want %d got %d", listSize, num)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.List()
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func BenchmarkListCRWithConvert(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, testNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkList(b, c)
}

func BenchmarkListCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, testNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkList(b, c)
}

func BenchmarkListEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkList(b, c)
}

func BenchmarkListEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, testNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkList(b, c)
}

func BenchmarkListCRWithConvert_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, testNamespace, foov1Template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c)
}

func BenchmarkListCR_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, testNamespace, barTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c)
}

func BenchmarkListEndpointsTyped_WatchCache(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(testNamespace, endpointsTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c)
}

func BenchmarkListEndpointsDynamic_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, testNamespace, endpointsTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c)
}

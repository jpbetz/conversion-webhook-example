package main

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GVR used for building dynamic client
	foov1GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	foov2GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v2", Resource: "foos"}
	barGVR       = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "bars"}
	endpointsGVR = schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}

	emptyNamespace         = "empty"
	largeDataNamespace     = "large-data"
	largeMetadataNamespace = "large-metadata"

	// size in kB
	largeDataSize = 10
	dummyFields   = []string{"spec", "dummy"}
	metaFields    = []string{"metadata", "annotations"}

	// number of objects we will create and list in list benchmarks
	emptyListSize         = 10000
	largeDataListSize     = 500
	largeMetadataListSize = 500
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

// TODO: TestMain actually runs after benchmarks, so this doesn't help yet
func TestMain(m *testing.M) {
	setupNamespace(emptyNamespace)
	setupNamespace(largeDataNamespace)
	setupNamespace(largeMetadataNamespace)
	os.Exit(m.Run())
}

func setupNamespace(name string) {
	c := mustNewClientset().CoreV1().Namespaces()
	_, err := c.Get(name, metav1.GetOptions{})
	if err == nil {
		return
	}
	if errors.IsNotFound(err) {
		if _, err := c.Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}); err != nil {
			panic(err)
		}
		// wait for namespace to be initialized
		time.Sleep(5 * time.Second)
	}
	panic(err)
}

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
	c := mustNewDynamicBenchmarkClient(foov1GVR, emptyNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, emptyNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCRWithConvert_LargeData(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeDataNamespace, template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCRWithConvert_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCR_LargeData(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeDataNamespace, template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyCR_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsTyped_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewEndpointsBenchmarkClient(largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkCreateLatency(b, c)
}

func BenchmarkCreateLatencyEndpointsDynamic_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(endpointsGVR, largeMetadataNamespace, template, &metav1.ListOptions{})
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
				// TODO: b.Fatal doesn't raise
				// b.Fatalf("failed to create object: %v", err)
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Printf("created %d objects in %v\n", count, time.Now().Sub(start))
}

func BenchmarkCreateThroughputCRWithConvert(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, emptyNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, emptyNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func BenchmarkCreateThroughputEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkCreateThroughput(b, c)
}

func benchmarkList(b *testing.B, client BenchmarkClient, listSize int) {
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
					// TODO: b.Fatal doesn't raise
					// b.Fatalf("failed to create object: %v", err)
					panic(err)
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
	c := mustNewDynamicBenchmarkClient(foov1GVR, emptyNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, emptyNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListEndpointsDynamic(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListCRWithConvert_LargeData(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeDataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeDataListSize)
}

func BenchmarkListCRWithConvert_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListCR_LargeData(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeDataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeDataListSize)
}

func BenchmarkListCR_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListEndpointsTyped_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewEndpointsBenchmarkClient(largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListEndpointsDynamic_LargeMetadata(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(endpointsGVR, largeMetadataNamespace, template, &metav1.ListOptions{})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListCRWithConvert_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, emptyNamespace, foov1Template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListCR_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, emptyNamespace, barTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListEndpointsTyped_WatchCache(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(emptyNamespace, endpointsTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListEndpointsDynamic_WatchCache(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(endpointsGVR, emptyNamespace, endpointsTemplate, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, emptyListSize)
}

func BenchmarkListCRWithConvert_LargeData_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeDataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeDataListSize)
}

func BenchmarkListCRWithConvert_LargeMetadata_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(foov1Template, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(foov1GVR, largeMetadataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListCR_LargeData_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, dummyFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeDataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeDataListSize)
}

func BenchmarkListCR_LargeMetadata_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(barTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(barGVR, largeMetadataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListEndpointsTyped_LargeMetadata_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewEndpointsBenchmarkClient(largeMetadataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeMetadataListSize)
}

func BenchmarkListEndpointsDynamic_LargeMetadata_WatchCache(b *testing.B) {
	template := mustIncreaseObjectSize(endpointsTemplate, largeDataSize, metaFields...)
	c := mustNewDynamicBenchmarkClient(endpointsGVR, largeMetadataNamespace, template, &metav1.ListOptions{ResourceVersion: "0"})
	benchmarkList(b, c, largeMetadataListSize)
}

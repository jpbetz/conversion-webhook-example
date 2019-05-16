package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	clientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var (
	// GVR used for building dynamic client
	foov1GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "foos"}
	foov2GVR     = schema.GroupVersionResource{Group: "stable.example.com", Version: "v2", Resource: "foos"}
	barGVR       = schema.GroupVersionResource{Group: "stable.example.com", Version: "v1", Resource: "bars"}
	endpointsGVR = schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}
	notfoundGVR  = schema.GroupVersionResource{Version: "error", Resource: "notfound"}

	emptyNamespace         = "empty"
	largeDataNamespace     = "large-data"
	largeMetadataNamespace = "large-metadata"

	fooName = "foos.stable.example.com"
	barName = "bars.stable.example.com"

	// size in kB
	largeDataSize = 10
	dummyFields   = []string{"spec", "dummy"}
	metaFields    = []string{"metadata", "annotations"}

	// number of objects we will create and list in list benchmarks
	testListSize = 10000
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

var validationSchema = []byte(`openAPIV3Schema:
  type: object
  properties:
    spec:
      type: object
      properties:
        dummy:
          description: Dummy array.
          type: array
          items:
            type: string
            pattern: dummy-[0-9]+
    status:
      type: object
      properties:
        baz:
          description: Optional Baz.`)

// TODO: TestMain actually runs after benchmarks, so this doesn't help yet
// func TestMain(m *testing.M) {
// 	setupNamespace(emptyNamespace)
// 	setupNamespace(largeDataNamespace)
// 	setupNamespace(largeMetadataNamespace)
// 	os.Exit(m.Run())
// }

func runBenchmark(b *testing.B) {
	// TODO: this is a workaround for go-benchmark not supporting before-benchmark setup
	setupNamespace(emptyNamespace)
	setupNamespace(largeDataNamespace)
	setupNamespace(largeMetadataNamespace)

	// get caller name
	pc, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc).Name()
	setupValidation(strings.Contains(caller, "Validation"))

	var c BenchmarkClient
	if strings.Contains(caller, "Typed") {
		c = mustNewEndpointsBenchmarkClient(getNamespace(caller), getTemplate(caller), getListOptions(caller))
	} else {
		c = mustNewDynamicBenchmarkClient(getGVR(caller), getNamespace(caller), getTemplate(caller), getListOptions(caller))
	}

	if strings.Contains(caller, "CreateLatency") {
		benchmarkCreateLatency(b, c)
	} else if strings.Contains(caller, "CreateThroughput") {
		benchmarkCreateThroughput(b, c)
	} else if strings.Contains(caller, "List") {
		benchmarkList(b, c, testListSize)
	}
}

func getGVR(name string) schema.GroupVersionResource {
	if strings.Contains(name, "CRWithConvert") {
		return foov1GVR
	}
	if strings.Contains(name, "CR") {
		return barGVR
	}
	if strings.Contains(name, "Endpoints") && strings.Contains(name, "Dynamic") {
		return endpointsGVR
	}
	return notfoundGVR
}

func getNamespace(name string) string {
	if strings.Contains(name, "LargeData") {
		return largeDataNamespace
	}
	if strings.Contains(name, "LargeMetadata") {
		return largeMetadataNamespace
	}
	return emptyNamespace
}

func getTemplate(name string) []byte {
	var template []byte
	if strings.Contains(name, "CRWithConvert") {
		template = foov1Template
	} else if strings.Contains(name, "CR") {
		template = barTemplate
	} else {
		template = endpointsTemplate
	}

	if strings.Contains(name, "LargeData") {
		template = mustIncreaseObjectSize(template, largeDataSize, dummyFields...)
	} else if strings.Contains(name, "LargeMetadata") {
		template = mustIncreaseObjectSize(template, largeDataSize, metaFields...)
	}
	return template
}

func getListOptions(name string) *metav1.ListOptions {
	if strings.Contains(name, "WatchCache") {
		return &metav1.ListOptions{ResourceVersion: "0"}
	}
	return &metav1.ListOptions{}
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
		time.Sleep(10 * time.Second)
	}
	panic(err)
}

func setupValidation(enable bool) {
	clientset, err := apiextensionsclientset.NewForConfig(mustNewRESTConfig())
	if err != nil {
		panic(err)
	}
	client := clientset.ApiextensionsV1beta1().CustomResourceDefinitions()
	if enable {
		v := v1beta1.CustomResourceValidation{}
		if err := yaml.Unmarshal(validationSchema, &v); err != nil {
			panic(err)
		}
		mustHaveValidation(client, fooName, &v)
		mustHaveValidation(client, barName, &v)
	} else {
		mustHaveValidation(client, fooName, nil)
		mustHaveValidation(client, barName, nil)
	}
}

// mustHaveValidation makes sure given CRD has expected validation set / unset
func mustHaveValidation(client clientv1beta1.CustomResourceDefinitionInterface, name string, validation *v1beta1.CustomResourceValidation) {
	crd, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	if apiequality.Semantic.DeepEqual(validation, crd.Spec.Validation) {
		return
	}
	crd.Spec.Validation = validation
	if _, err := client.Update(crd); err != nil {
		panic(err)
	}
	// wait for potential initialization
	time.Sleep(5 * time.Second)
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

func Benchmark_CreateLatency_CRWithConvert(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CRWithConvert_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CRWithConvert_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CRWithConvert_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CRWithConvert_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CRWithConvert_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_CR_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_Endpoints_Typed(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_Endpoints_Typed_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_Endpoints_Dynamic(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateLatency_Endpoints_Dynamic_LargeMetadata(b *testing.B) {
	runBenchmark(b)
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

func Benchmark_CreateThroughput_CRWithConvert(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateThroughput_CRWithConvert_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateThroughput_CR(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateThroughput_CR_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateThroughput_Endpoints_Typed(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_CreateThroughput_Endpoints_Dynamic(b *testing.B) {
	runBenchmark(b)
}

func ensureObjectCount(client BenchmarkClient, listSize int) error {
	num, err := client.Count()
	if err != nil {
		return fmt.Errorf("failed to check list size: %v", err)
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
		return fmt.Errorf("Too many items already exist. Want %d got %d", listSize, num)
	}
	return nil
}

func benchmarkList(b *testing.B, client BenchmarkClient, listSize int) {
	if err := ensureObjectCount(client, listSize); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.List()
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

func Benchmark_List_CRWithConvert(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CRWithConvert_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CRWithConvert_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CRWithConvert_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CRWithConvert_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CRWithConvert_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_CR_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_Endpoints_Typed(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_Endpoints_Dynamic(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_Endpoints_Typed_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_Endpoints_Dynamic_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CRWithConvert_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR_Validation(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR_Validation_LargeData(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_CR_Validation_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_Endpoints_Typed(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_Endpoints_Dynamic(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_Endpoints_Typed_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func Benchmark_List_WatchCache_Endpoints_Dynamic_LargeMetadata(b *testing.B) {
	runBenchmark(b)
}

func benchmarkWatch(b *testing.B, client BenchmarkClient, listSize int) {
	watcherCount := 1000
	events := b.N
	var readyWg sync.WaitGroup
	var doneWg sync.WaitGroup
	readyWg.Add(watcherCount)
	doneWg.Add(watcherCount)
	start := time.Now()
	for i := 0; i < watcherCount; i++ {
		go func() {
			watcher, err := client.Watch()
			if err != nil {
				b.Fatalf("failed to watch: %v", err)
			}
			readyWg.Done()
			for j := 0; j < events; j++ {
				<-watcher.ResultChan()
			}
			watcher.Stop()
			doneWg.Done()
		}()
	}
	readyWg.Wait()
	fmt.Printf("created %d watches in %v\n", watcherCount, time.Now().Sub(start))
	start = time.Now()
	b.ResetTimer()
	for i := 0; i < events; i++ {
		go func() {
			client.Create(i)
		}()
	}
	doneWg.Wait()
	fmt.Printf("processed %d watch events in %v\n", watcherCount*events, time.Now().Sub(start))
}

func BenchmarkWatchCRWithConvert(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(foov1GVR, emptyNamespace, foov1Template, &metav1.ListOptions{})
	benchmarkWatch(b, c, testListSize)
}

func BenchmarkWatchCR(b *testing.B) {
	c := mustNewDynamicBenchmarkClient(barGVR, emptyNamespace, barTemplate, &metav1.ListOptions{})
	benchmarkWatch(b, c, testListSize)
}

func BenchmarkWatchEndpointsTyped(b *testing.B) {
	c := mustNewEndpointsBenchmarkClient(emptyNamespace, endpointsTemplate, &metav1.ListOptions{})
	benchmarkWatch(b, c, testListSize)
}

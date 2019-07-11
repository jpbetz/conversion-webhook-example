package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	clientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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
	largeDataSize = 50
	dummyFields   = []string{"spec", "dummy"}
	metaFields    = []string{"metadata", "annotations"}

	// number of objects we will create and list in list benchmarks
	testListSize = 1000
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
    host:
      type: string
    port:
      type: string
    hostPort:
      type: string
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
          type: object
          description: Optional Baz.`)

// mustNewRESTConfig builds a rest client config
func mustNewRESTConfig() *rest.Config {
	// TODO: add flag support in TestMain for running in master VM / remotely
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	// config, err := clientcmd.DefaultClientConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	// wait for long running requests, e.g. deleting 10k objects
	config.Timeout = 10 * time.Minute

	// increase QPS (default 5) for heavy load testing
	config.QPS = 10000
	config.Burst = 20000
	return config
}

// mustNewDynamicClient creates a new dynamic client
func mustNewDynamicClient() dynamic.Interface {
	client, err := dynamic.NewForConfig(mustNewRESTConfig())
	if err != nil {
		panic(err)
	}
	return client
}

// mustNewClientset creates a new clientset containing typed clients for groups
func mustNewClientset() *kubernetes.Clientset {
	client, err := kubernetes.NewForConfig(mustNewRESTConfig())
	if err != nil {
		panic(err)
	}
	return client
}

// BenchmarkClient provides create and list interface for benchmark testing
type BenchmarkClient interface {
	// use i to customize and avoid race
	Create(i int) (interface{}, error)
	List() (interface{}, error)
	Count() (int, error)
	Watch() (watch.Interface, error)
	DeleteCollection() error
}

var _ BenchmarkClient = &dynamicBenchmarkClient{}
var _ BenchmarkClient = &endpointsBenchmarkClient{}

// dynamicBenchmarkClient implements BenchmarkClient interface
type dynamicBenchmarkClient struct {
	client      dynamic.ResourceInterface
	template    *unstructured.Unstructured
	listOptions *metav1.ListOptions
}

func (c *dynamicBenchmarkClient) Create(i int) (interface{}, error) {
	obj := c.template.DeepCopy()
	obj.SetName(fmt.Sprintf("%d-%d", time.Now().Nanosecond(), i))
	return c.client.Create(obj, metav1.CreateOptions{})
}

func (c *dynamicBenchmarkClient) List() (interface{}, error) {
	return c.client.List(*c.listOptions)
}

func (c *dynamicBenchmarkClient) Count() (int, error) {
	l, err := c.client.List(metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(l.Items), nil
}

func (c *dynamicBenchmarkClient) Watch() (watch.Interface, error) {
	return c.client.Watch(*c.listOptions)
}

func (c *dynamicBenchmarkClient) DeleteCollection() error {
	return c.client.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
}

// endpointsBenchmarkClient implements BenchmarkClient interface
type endpointsBenchmarkClient struct {
	client      clientv1.EndpointsInterface
	template    *v1.Endpoints
	listOptions *metav1.ListOptions
}

func (c *endpointsBenchmarkClient) Create(i int) (interface{}, error) {
	obj := c.template.DeepCopy()
	obj.SetName(fmt.Sprintf("%d-%d", time.Now().Nanosecond(), i))
	return c.client.Create(obj)
}

func (c *endpointsBenchmarkClient) List() (interface{}, error) {
	return c.client.List(*c.listOptions)
}

func (c *endpointsBenchmarkClient) Count() (int, error) {
	// list from etcd
	l, err := c.client.List(metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(l.Items), nil
}

func (c *endpointsBenchmarkClient) Watch() (watch.Interface, error) {
	return c.client.Watch(*c.listOptions)
}

func (c *endpointsBenchmarkClient) DeleteCollection() error {
	return c.client.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
}

func mustNewDynamicBenchmarkClient(gvr schema.GroupVersionResource, namespace string,
	templateData []byte, listOptions *metav1.ListOptions) BenchmarkClient {
	template := unstructured.Unstructured{}
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		panic(err)
	}
	return &dynamicBenchmarkClient{
		client:      mustNewDynamicClient().Resource(gvr).Namespace(namespace),
		template:    &template,
		listOptions: listOptions,
	}
}

func mustNewEndpointsBenchmarkClient(namespace string, templateData []byte,
	listOptions *metav1.ListOptions) BenchmarkClient {
	template := v1.Endpoints{}
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		panic(err)
	}
	return &endpointsBenchmarkClient{
		client:      mustNewClientset().CoreV1().Endpoints(namespace),
		template:    &template,
		listOptions: listOptions,
	}
}

// mustIncreaseObjectSize bumps data by kB size
func mustIncreaseObjectSize(data []byte, size int, fields ...string) []byte {
	u := unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &u); err != nil {
		panic(err)
	}
	// NOTE: we are have a rough equivalence in size between annotation and CR array,
	// because there is no good array candidate in metadata
	if fields[0] == "metadata" {
		dummy := map[string]string{}
		for i := 0; i < size; i++ {
			// TODO: double check wired format size
			// 1000 bytes each in JSON (10+6+984=1000)
			//     ,"<10 bytes>":"<984 bytes>"
			dummy[fmt.Sprintf("%010d", i)] = strings.Repeat("x", 984)
		}
		if err := unstructured.SetNestedStringMap(u.Object, dummy, fields...); err != nil {
			panic(err)
		}
	} else {
		// TODO: unstructured doesn't support set (deep copy) nested struct
		dummy := []string{}
		for i := 0; i < size; i++ {
			// TODO: double check wired format size
			// 1000 bytes each in JSON (9+991=1000)
			//     ,"dummy-<991 bytes>"
			dummy = append(dummy, fmt.Sprintf("dummy-%0991d", i))
		}
		if err := unstructured.SetNestedStringSlice(u.Object, dummy, fields...); err != nil {
			panic(err)
		}
	}
	d, err := yaml.Marshal(&u)
	if err != nil {
		panic(err)
	}
	return d
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
	} else {
		panic(err)
	}
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

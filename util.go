package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

// mustNewRESTConfig builds a rest client config
func mustNewRESTConfig() *rest.Config {
	// TODO: add flag / in-cluster config support
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	// increase QPS (default 5) for heavy load testing
	config.QPS = 1000.0
	config.Burst = 2000
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
	d, err := yaml.Marshal(&u)
	if err != nil {
		panic(err)
	}
	return d
}

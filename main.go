package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/jamiealquiza/tachymeter"
)

func main() {
	name := flag.String("name", "", "TODO: documentation")
	run := flag.Int("run", 100, "TODO: documentation")
	window := flag.Int("window", 50, "TODO: documentation")
	flag.Parse()
	caller := *name
	fmt.Println(caller)

	// set up env
	setupNamespace(emptyNamespace)
	setupNamespace(largeDataNamespace)
	setupNamespace(largeMetadataNamespace)
	setupValidation(strings.Contains(caller, "Validation"))

	var c BenchmarkClient
	if strings.Contains(caller, "Typed") {
		c = mustNewEndpointsBenchmarkClient(getNamespace(caller), getTemplate(caller), getListOptions(caller))
	} else {
		c = mustNewDynamicBenchmarkClient(getGVR(caller), getNamespace(caller), getTemplate(caller), getListOptions(caller))
	}

	// always delete all objects created by current run, to avoid overwhelm etcd over time
	defer func() {
		if err := c.DeleteCollection(); err != nil {
			panic(fmt.Errorf("failed to clean up objects: %v", err))
		}
		fmt.Println("objects cleaned up")
	}()

	if strings.Contains(caller, "List") {
		if err := ensureObjectCount(c, testListSize); err != nil {
			panic(err)
		}
		fmt.Println("enough objects prepared")
	}

	// actual measurement
	t := tachymeter.New(&tachymeter.Config{Size: *window})

	var err error
	for i := 0; i < *run; i++ {
		start := time.Now()

		// TODO: error on unsupported case
		if strings.Contains(caller, "CreateLatency") {
			_, err = c.Create(0)
		} else if strings.Contains(caller, "List") {
			_, err = c.List()
		}
		if err != nil {
			panic(err)
		}

		t.AddTime(time.Since(start))
	}

	fmt.Println(t.Calc().String())
}

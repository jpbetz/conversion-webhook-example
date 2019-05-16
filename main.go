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

	if strings.Contains(caller, "List") {
		if err := ensureObjectCount(c, testListSize); err != nil {
			panic(err)
		}
	}

	// actual measurement
	t := tachymeter.New(&tachymeter.Config{Size: *window})

	var err error
	for i := 0; i < *run; i++ {
		start := time.Now()

		// TODO: error on unsupported case
		if strings.Contains(caller, "CreateLatency") {
			_, err = client.Create(0)
		} else if strings.Contains(caller, "List") {
			_, err = client.List()
		}
		if err != nil {
			panic(err)
		}

		t.AddTime(time.Since(start))
	}

	fmt.Println(t.Calc().String())
}

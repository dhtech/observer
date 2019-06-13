package main

import (
	"fmt"
	"time"
	"flag"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	iface = flag.String("i", "eth0", "Interface to configure via DHCP")
	interval = flag.Duration("s", time.Second * 5, "Collection interval")
	verbose = flag.Bool("v", false, "Verbose")
)

func main() {
	flag.Parse()
	fmt.Println("Starting observer.")
	http.Handle("/metrics", promhttp.Handler())

	go func(){
		for {
			sampleDhcp(*iface, *verbose)
			time.Sleep(*interval)
		}
	}()

	err := http.ListenAndServe(":9023", nil)
	if err != nil {
		panic(err)
	}
}
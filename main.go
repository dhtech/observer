package main

import (
	"fmt"
	"time"
	"flag"
	"strings"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	iface = flag.String("i", "eth0", "Interface to configure via DHCP")
	dnsTargets = flag.String("d", "", "DNS servers to probe, separate by comma.")
	dnsQname = flag.String("r", "healthcheck.event.dreamhack.se.", "DNS healthcheck qname to probe.")
	interval = flag.Duration("s", time.Second * 5, "Collection interval")
	verbose = flag.Bool("v", false, "Verbose")
)

func main() {
	flag.Parse()
	fmt.Println("Starting observer.")
	if *dnsTargets == "" {
		panic("No DNS targets supplied")
	}
	dnsTargetsList := strings.Split(*dnsTargets, ",")
	http.Handle("/metrics", promhttp.Handler())

	go func(){
		for {
			sampleDhcp(*iface, *verbose)
			sampleDns(dnsTargetsList, *dnsQname)
			time.Sleep(*interval)
		}
	}()

	err := http.ListenAndServe(":9023", nil)
	if err != nil {
		panic(err)
	}
}
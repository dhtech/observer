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
	iface = flag.String("interface", "eth0", "Interface to configure via DHCP")
	dnsTargets = flag.String("dns", "", "DNS servers to probe, separate by comma.")
	icmpTargets = flag.String("icmp", "", "ICMP target.")
	icmpCount = flag.Int("icmp-count", 3, "ICMP count.")
	dnsQname = flag.String("qname", "healthcheck.event.dreamhack.se.", "DNS healthcheck qname to probe.")
	interval = flag.Duration("interval", time.Second * 5, "Collection interval")
	verbose = flag.Bool("verbose", false, "Verbose")
)

func main() {
	flag.Parse()
	fmt.Println("Starting observer.")
	dnsTargetsList := strings.Split(*dnsTargets, ",")
	icmpTargetsList := strings.Split(*icmpTargets, ",")
	http.Handle("/metrics", promhttp.Handler())

	go func(){
		for {
			sampleDhcp(*iface, *verbose)
			if len(dnsTargetsList) > 0 {
				sampleDns(dnsTargetsList, *dnsQname)
			}
			if len(icmpTargetsList) > 0 {
				sampleIcmp(icmpTargetsList, *icmpCount)
			}
			time.Sleep(*interval)
		}
	}()

	err := http.ListenAndServe(":9023", nil)
	if err != nil {
		panic(err)
	}
}
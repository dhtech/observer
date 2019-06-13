package main

import (
	"fmt"
	"time"

	dhcp "github.com/insomniacslk/dhcp/dhcpv4"
	client "github.com/insomniacslk/dhcp/dhcpv4/client4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dhcpRequests = promauto.NewCounter(prometheus.CounterOpts{
			Name: "observer_dhcp_requests",
			Help: "Total number of sent DHCP requests",
	})
	dhcpOffers = promauto.NewCounter(prometheus.CounterOpts{
			Name: "observer_dhcp_offers",
			Help: "Total number of received DHCP offers",
	})
	dhcpFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp_failures",
		Help: "Total number of failed DHCP handshakes",
	})
	dchpHandshakeLatency = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp_latency",
		Help: "Time between dhcp request and dhcp offer.",
	})
	dchpLeaseTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp_lease_time",
		Help: "Time until DHCP lease will be renewed.",
	})
)

func init() {
	prometheus.MustRegister(dchpHandshakeLatency)
	prometheus.MustRegister(dchpLeaseTime)
}

func sampleDhcp(iface string, verbose bool) {
	dhcpRequests.Inc()
	client := client.NewClient()
	start := time.Now()
	conversation, err := client.Exchange(iface)
	if err != nil {
		dhcpFailures.Inc()
		fmt.Printf("DHCP Failed with %s.\n", err)
		return
	}
	dchpHandshakeLatency.Set(time.Since(start).Seconds())
	for _, packet := range conversation {
	 	if packet.MessageType() ==  dhcp.MessageTypeOffer {
			dhcpOffers.Inc()
		}
		if packet.Options.Has(dhcp.OptionIPAddressLeaseTime) {
			dchpLeaseTime.Set(packet.IPAddressLeaseTime(0).Seconds())
		}
		if verbose {
			fmt.Println(packet.Summary())
		}
	}
}
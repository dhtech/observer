package main

import (
	"fmt"
	"net"
	"time"

	dhcp "github.com/insomniacslk/dhcp/dhcpv4"
	client "github.com/insomniacslk/dhcp/dhcpv4/client4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dhcpRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp_requests",
		Help: "Total number of sent DHCPv4 requests",
	})
	dhcpOffers = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp_offers",
		Help: "Total number of received DHCPv4 offers",
	})
	dhcpFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp_failures",
		Help: "Total number of failed DHCPv4 handshakes",
	})
	dhcpHandshakeLatency = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp_latency",
		Help: "Time between DHCPv4 request and offer.",
	})
	dhcpLeaseTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp_lease_time",
		Help: "Time until DHCPv4 lease will be renewed.",
	})
)

func init() {
	prometheus.MustRegister(dhcpHandshakeLatency)
	prometheus.MustRegister(dhcpLeaseTime)
}

func sampleDhcp(iface string, verbose bool) (net.IP, int, error) {
	dhcpRequests.Inc()
	client := client.NewClient()
	start := time.Now()
	conversation, err := client.Exchange(iface)
	if err != nil {
		dhcpFailures.Inc()
		return nil, 0, err
	}
	dhcpHandshakeLatency.Set(time.Since(start).Seconds())

	var yourIPAddr net.IP
	var prefixBits int
	for _, packet := range conversation {
		if verbose {
			fmt.Println(packet.Summary())
		}

		if packet.MessageType() == dhcp.MessageTypeOffer {
			dhcpOffers.Inc()
		}

		if packet.MessageType() == dhcp.MessageTypeAck {
			if packet.Options.Has(dhcp.OptionIPAddressLeaseTime) {
				dhcpLeaseTime.Set(packet.IPAddressLeaseTime(0).Seconds())
			}

			prefixBits, _ = packet.SubnetMask().Size()
			yourIPAddr = packet.YourIPAddr
		}
	}
	return yourIPAddr, prefixBits, err
}

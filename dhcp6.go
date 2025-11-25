package main

import (
	"fmt"
	"net"
	"time"

	dhcp "github.com/insomniacslk/dhcp/dhcpv6"
	client "github.com/insomniacslk/dhcp/dhcpv6/client6"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dhcp6Requests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp6_requests",
		Help: "Total number of sent DHCPv6 requests",
	})
	dhcp6Replies = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp6_offers",
		Help: "Total number of received DHCPv6 replies(offers)",
	})
	dhcp6Failures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observer_dhcp6_failures",
		Help: "Total number of failed DHCPv6 handshakes",
	})
	dhcp6HandshakeLatency = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp6_latency",
		Help: "Time between DHCPv6 request and offer.",
	})
	dhcp6Lifetime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "observer_dhcp6_lifetime",
		Help: "Duration that DHCPv6 obtained address is valid.",
	})
)

func init() {
	prometheus.MustRegister(dhcp6HandshakeLatency)
	prometheus.MustRegister(dhcp6Lifetime)
}

func sampleDhcp6(iface string, verbose bool) (net.IP, int, error) {
	dhcp6Requests.Inc()
	client := client.NewClient()
	start := time.Now()
	conversation, err := client.Exchange(iface)
	if err != nil {
		dhcp6Failures.Inc()
		return nil, 0, err
	}
	dhcp6HandshakeLatency.Set(time.Since(start).Seconds())

	var yourIPAddr net.IP
	var prefixBits = 128
	for _, packet := range conversation {
		if verbose {
			fmt.Println(packet.Summary())
		}

		message, err := packet.GetInnerMessage()
		if err != nil {
			dhcp6Failures.Inc()
			return nil, 0, err
		}

		if message.MessageType == dhcp.MessageTypeReply {
			dhcp6Replies.Inc()

			iaNa := message.Options.OneIANA()
			naAddr := *iaNa.Options.OneAddress()

			dhcp6Lifetime.Set(naAddr.ValidLifetime.Seconds())
			yourIPAddr = naAddr.IPv6Addr
		}
	}
	return yourIPAddr, prefixBits, err
}

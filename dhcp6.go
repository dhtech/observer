package main

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/client6"
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

func sampleDhcp6(client client6.Client, verbose bool) (net.IP, int, error) {
	dhcp6Requests.Inc()
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

		if message.MessageType == dhcpv6.MessageTypeReply {
			dhcp6Replies.Inc()

			naAddr, err := addressFromReply(message)
			if err != nil {
				dhcp6Failures.Inc()
				return nil, 0, err
			}

			dhcp6Lifetime.Set(naAddr.ValidLifetime.Seconds())
			yourIPAddr = naAddr.IPv6Addr
		}
	}
	return yourIPAddr, prefixBits, nil
}

// addressFromReply returns the address a DHCPv6 reply gives. A server
// that has none to give answers without one: that is a failed
// handshake, told as an error.
func addressFromReply(message *dhcpv6.Message) (*dhcpv6.OptIAAddress, error) {
	iaNa := message.Options.OneIANA()
	if iaNa == nil {
		return nil, errors.New("DHCPv6 reply holds no IA_NA")
	}
	naAddr := iaNa.Options.OneAddress()
	if naAddr == nil {
		return nil, errors.New("DHCPv6 reply holds no address")
	}
	return naAddr, nil
}

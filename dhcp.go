package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
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

// sampleDhcp makes one whole DHCPv4 handshake and returns the address
// and the prefix length the server gave. Every message is waited for
// no longer than timeout.
func sampleDhcp(timeout time.Duration, verbose bool) (net.IP, int, error) {
	dhcpRequests.Inc()
	start := time.Now()
	ack, err := exchangeDhcp(timeout, verbose)
	if err != nil {
		dhcpFailures.Inc()
		return nil, 0, err
	}
	dhcpHandshakeLatency.Set(time.Since(start).Seconds())

	if ack.Options.Has(dhcpv4.OptionIPAddressLeaseTime) {
		dhcpLeaseTime.Set(ack.IPAddressLeaseTime(0).Seconds())
	}

	yourIPAddr, prefixBits := addressOfAck(ack)
	return yourIPAddr, prefixBits, nil
}

// exchangeDhcp sends a discover, takes the offer, requests it and
// returns the server's acknowledgement. It uses the library's nclient4:
// the older client4 gave up a handshake whenever its read was
// interrupted by a signal ("interrupted system call"), which the Go
// runtime does to its own threads now and then.
func exchangeDhcp(timeout time.Duration, verbose bool) (*dhcpv4.DHCPv4, error) {
	// one try for every message, as before: a round must not grow long
	opts := []nclient4.ClientOpt{nclient4.WithTimeout(timeout), nclient4.WithRetry(1)}
	if verbose {
		opts = append(opts, nclient4.WithDebugLogger())
	}
	client, err := nclient4.New(iface, opts...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	offer, err := client.DiscoverOffer(ctx)
	if err != nil {
		return nil, fmt.Errorf("no offer: %w", err)
	}
	dhcpOffers.Inc()

	lease, err := client.RequestFromOffer(ctx, offer)
	if err != nil {
		return nil, fmt.Errorf("no acknowledgement: %w", err)
	}
	return lease.ACK, nil
}

// addressOfAck returns the address an acknowledgement gives and the
// length of its network's prefix.
func addressOfAck(ack *dhcpv4.DHCPv4) (net.IP, int) {
	prefixBits, _ := ack.SubnetMask().Size()
	return ack.YourIPAddr, prefixBits
}

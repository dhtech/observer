package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
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

// sampleDhcp6 makes one whole stateful DHCPv6 handshake and returns the
// address the server gave. Every message is waited for no longer than
// timeout.
func sampleDhcp6(timeout time.Duration, verbose bool) (net.IP, int, error) {
	dhcp6Requests.Inc()
	start := time.Now()
	reply, err := exchangeDhcp6(timeout, verbose)
	if err != nil {
		dhcp6Failures.Inc()
		return nil, 0, err
	}
	dhcp6HandshakeLatency.Set(time.Since(start).Seconds())
	dhcp6Replies.Inc()

	naAddr, err := addressFromReply(reply)
	if err != nil {
		dhcp6Failures.Inc()
		return nil, 0, err
	}

	dhcp6Lifetime.Set(naAddr.ValidLifetime.Seconds())
	return naAddr.IPv6Addr, 128, nil
}

// exchangeDhcp6 sends a solicit, takes the advertise, requests it and
// returns the server's reply. It uses the library's nclient6, the
// newer client, as the DHCPv4 check uses nclient4.
func exchangeDhcp6(timeout time.Duration, verbose bool) (*dhcpv6.Message, error) {
	// one try for every message, as before: a round must not grow long
	opts := []nclient6.ClientOpt{nclient6.WithTimeout(timeout), nclient6.WithRetry(1)}
	if verbose {
		opts = append(opts, nclient6.WithDebugLogger())
	}
	client, err := nclient6.New(iface, opts...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	advertise, err := client.Solicit(ctx)
	if err != nil {
		return nil, fmt.Errorf("no advertise: %w", err)
	}

	reply, err := client.Request(ctx, advertise)
	if err != nil {
		return nil, fmt.Errorf("no reply: %w", err)
	}
	if reply.MessageType != dhcpv6.MessageTypeReply {
		return nil, fmt.Errorf("the request was answered with a %s, not a reply", reply.MessageType)
	}
	return reply, nil
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

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// icmpMetrics are the numbers of the ping check over one address family.
type icmpMetrics struct {
	setupFailures *prometheus.CounterVec
	avgRtt        *prometheus.GaugeVec
	sent          *prometheus.CounterVec
	received      *prometheus.CounterVec
}

// newIcmpMetrics makes the metrics observer_..icmp<family>.. for family
// "4" or "6".
func newIcmpMetrics(family string) icmpMetrics {
	return icmpMetrics{
		setupFailures: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_icmp" + family + "_setup_failures",
			Help: "Total number of failed ICMPv" + family + " tests due to errors setting up the measurement",
		}, []string{"target"}),
		avgRtt: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "observer_icmp" + family + "_avg_rtt",
			Help: "Average round trip time of the last ICMPv" + family + " requests.",
		}, []string{"target", "resolved_addr"}),
		sent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_sent_icmp" + family + "_requests",
			Help: "Total number of sent ICMPv" + family + " echo requests",
		}, []string{"target", "resolved_addr"}),
		received: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_received_icmp" + family + "_requests",
			Help: "Total number of received ICMPv" + family + " echo replies",
		}, []string{"target", "resolved_addr"}),
	}
}

var (
	icmp4Metrics = newIcmpMetrics("4")
	icmp6Metrics = newIcmpMetrics("6")
)

// resolveTarget returns the addresses of a ping target: the address
// itself, or every address its name has.
func resolveTarget(target string) ([]net.IP, error) {
	if address := net.ParseIP(target); address != nil {
		return []net.IP{address}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), interval)
	defer cancel()
	return net.DefaultResolver.LookupIP(ctx, "ip", target)
}

// addressOf returns the first IPv4 address of the list - with v6, the
// first IPv6 one - and nil when it has none of that family.
func addressOf(addresses []net.IP, v6 bool) net.IP {
	for _, address := range addresses {
		if (address.To4() == nil) == v6 {
			return address
		}
	}
	return nil
}

// sampleIcmp pings every target over the address families asked for
// (v4, v6) that the target has an address of, also after one that
// failed, and returns what failed.
func sampleIcmp(targets []string, count int, v4, v6 bool) error {
	var errs []error
	for _, target := range targets {
		addresses, err := resolveTarget(target)
		if err != nil {
			// not looked up: a failure of every family it would be pinged over
			labels := prometheus.Labels{"target": target}
			if v4 {
				icmp4Metrics.setupFailures.With(labels).Inc()
			}
			if v6 {
				icmp6Metrics.setupFailures.With(labels).Inc()
			}
			errs = append(errs, fmt.Errorf("%s: %w", target, err))
			continue
		}
		if address := addressOf(addresses, false); v4 && address != nil {
			errs = append(errs, ping(icmp4Metrics, target, address, count))
		}
		if address := addressOf(addresses, true); v6 && address != nil {
			errs = append(errs, ping(icmp6Metrics, target, address, count))
		}
	}

	return errors.Join(errs...)
}

// ping sends count echo requests to one address of a target and
// counts what came back.
func ping(metrics icmpMetrics, target string, address net.IP, count int) error {
	f := metrics.setupFailures.With(prometheus.Labels{
		"target": target,
	})
	f.Add(float64(0))
	pinger, err := probing.NewPinger(address.String())
	if err != nil {
		f.Inc()
		return fmt.Errorf("%s: %w", target, err)
	}
	pinger.SetPrivileged(true)
	pinger.Count = count
	// an answer that does not come is not waited for without end:
	// every packet, also the last one, gets one interval
	pinger.Timeout = time.Duration(count) * pinger.Interval
	pinger.InterfaceName = iface

	err = pinger.Run()
	if err != nil {
		f.Inc()
		return fmt.Errorf("%s: %w", target, err)
	}

	stats := pinger.Statistics()

	labels := prometheus.Labels{
		"target":        target,
		"resolved_addr": stats.IPAddr.String(),
	}

	metrics.sent.With(labels).Add(float64(stats.PacketsSent))

	metrics.received.With(labels).Add(float64(stats.PacketsRecv))

	// no answer at all has no round trip time: the last good one is
	// not shown on
	if stats.PacketsRecv == 0 {
		metrics.avgRtt.Delete(labels)
		return nil
	}

	metrics.avgRtt.With(labels).Set(stats.AvgRtt.Seconds())
	return nil
}

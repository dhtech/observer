package main

import (
	probing "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	icmpSetupFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_icmp_setup_failures",
		Help: "Total number of failed ICMP tests due to errors setting up the measurement",
	}, []string{"target"})
	icmpAvgRtt = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observer_icmp_avg_rtt",
		Help: "Average round trip time of ICMP request.",
	}, []string{"target", "resolved_addr"})
	icmpSentRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_sent_icmp_requests",
		Help: "Total number of sent ICMP echo requests",
	}, []string{"target", "resolved_addr"})
	icmpReceivedRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_received_icmp_requests",
		Help: "Total number of received ICMP echo replies",
	}, []string{"target", "resolved_addr"})
)

func init() {
	prometheus.MustRegister(icmpAvgRtt)
}

func sampleIcmp(targets []string, count int) {
	for _, target := range targets {
		f := icmpSetupFailures.With(prometheus.Labels{
			"target": target,
		})
		f.Add(float64(0))
		pinger, err := probing.NewPinger(target)
		if err != nil {
			f.Inc()
			return
		}
		pinger.SetPrivileged(true)
		pinger.Count = count
		pinger.InterfaceName = iface

		var af string
		if !disable4 {
			af = "ip6"
		} else if !disable6 {
			af = "ip4"
		} else {
			af = "ip"
		}
		pinger.SetNetwork(af)

		err = pinger.Run()
		if err != nil {
			f.Inc()
			return
		}

		stats := pinger.Statistics()

		icmpSentRequests.With(prometheus.Labels{
			"target":        target,
			"resolved_addr": stats.IPAddr.String(),
		}).Add(float64(stats.PacketsSent))

		icmpReceivedRequests.With(prometheus.Labels{
			"target":        target,
			"resolved_addr": stats.IPAddr.String(),
		}).Add(float64(stats.PacketsRecv))

		icmpAvgRtt.With(prometheus.Labels{
			"target":        target,
			"resolved_addr": stats.IPAddr.String(),
		}).Set(stats.AvgRtt.Seconds())
	}
}

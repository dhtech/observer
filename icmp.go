package main

import (
	"github.com/sparrc/go-ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	icmpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "icmp_requests",
			Help: "Total number of sent ICMP requests",
	}, []string{"target"})
	icmpFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icmp_failures",
		Help: "Total number of failed ICMP requests",
	}, []string{"target"})
	icmpRtt = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icmp_rtt",
		Help: "RTT of ICMP request.",
	}, []string{"target"})
)

func init() {
	prometheus.MustRegister(icmpRtt)
}

func sampleIcmp(targets []string, count int) {
	for _, target := range targets {
		icmpRequests.With(prometheus.Labels{
			"target": target,
		}).Add(float64(count))
		pinger, err := ping.NewPinger(target)
		pinger.SetPrivileged(true)
		if err != nil {
			icmpFailures.With(prometheus.Labels{
				"target": target,
			}).Inc()
			return
		}
		pinger.Count = count
		pinger.Run()
		stats := pinger.Statistics()
		if stats.PacketsSent != stats.PacketsRecv {
			icmpFailures.With(prometheus.Labels{
				"target": target,
			}).Inc()
		}
		icmpRtt.With(prometheus.Labels{
			"target": target,
		}).Set(float64(stats.AvgRtt.Nanoseconds()))
	}
	
}
package main

import (
	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	icmpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_icmp_requests",
			Help: "Total number of sent ICMP requests",
	}, []string{"target"})
	icmpFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_icmp_failures",
		Help: "Total number of failed ICMP requests",
	}, []string{"target"})
	icmpRtt = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observer_icmp_rtt",
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
		f := icmpFailures.With(prometheus.Labels{
			"target": target,
		})
		f.Add(float64(0))
		pinger, err := ping.NewPinger(target)
		pinger.SetPrivileged(true)
		if err != nil {
			f.Inc()
			return
		}
		pinger.Count = count
		pinger.Run()
		stats := pinger.Statistics()
		if stats.PacketsSent != stats.PacketsRecv {
			f.Inc()
		}
		icmpRtt.With(prometheus.Labels{
			"target": target,
		}).Set(stats.AvgRtt.Seconds())
	}
	
}

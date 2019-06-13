package main

import (
	"fmt"
	"time"
	"strconv"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dnsRequests = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "dns_client_requests",
			Help: "Total number of sent DNS requests",
	}, []string{"resolver"})
	dnsFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dns_client_failures",
		Help: "Total number of failed DNS requests",
	}, []string{"resolver"})
	dnsLatency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dns_client_latency",
		Help: "Latency of DNS reequest.",
	}, []string{"resolver"})
	dnsAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dns_client_age",
		Help: "Age of DNS healthcheck record.",
	}, []string{"resolver"})
)

func init() {
	prometheus.MustRegister(dnsLatency)
	prometheus.MustRegister(dnsAge)
}

func sampleDns(targets []string, qname string) {
	for _, target := range targets {
		c := dns.Client{}
		m := dns.Msg{}
		m.SetQuestion(qname, dns.TypeTXT)
		start := time.Now()
		r, _, err := c.Exchange(&m, fmt.Sprintf("%s:53", target))
		dnsRequests.With(prometheus.Labels{
			"resolver": target,
		}).Inc()
		if err != nil {
			dnsFailures.With(prometheus.Labels{
				"resolver": target,
			}).Inc()
			return
		}
		dnsLatency.With(prometheus.Labels{
			"resolver": target,
		}).Set(float64(time.Since(start).Nanoseconds()))
		if len(r.Answer) == 0 {
			return
		}
		for _, ans := range r.Answer {
			if t, ok := ans.(*dns.TXT); ok {
				if len(t.Txt) > 0 {
					stamp, err := strconv.Atoi(t.Txt[0])
					if err != nil {
						fmt.Println("Failed to parse healthcheck record")
						return
					}
					dnsAge.With(prometheus.Labels{
						"resolver": target,
					}).Set(float64(int(time.Now().Unix())-stamp))
				}
			}
		}
	}
}
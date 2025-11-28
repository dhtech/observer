package main

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dnsRequestsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_dns_requests",
		Help: "Total number of sent DNS requests",
	}, []string{"resolver"})
	dnsFailuresCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "observer_dns_failures",
		Help: "Total number of failed DNS requests",
	}, []string{"resolver"})
	dnsRttGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observer_dns_rtt",
		Help: "Latency of DNS reequest.",
	}, []string{"resolver"})
)

func init() {
	prometheus.MustRegister(dnsRttGauge)
}

func sampleDns(targets []string, qname string) error {
	for _, target := range targets {

		dnsFailures := dnsFailuresCounter.With(prometheus.Labels{
			"resolver": target,
		})

		dnsRequestsCounter.With(prometheus.Labels{
			"resolver": target,
		}).Inc()

		dnsClient := dns.Client{}
		dnsMessage := dns.Msg{}
		dnsMessage.SetQuestion(qname, dns.TypeTXT)

		r, rtt, err := dnsClient.Exchange(&dnsMessage, fmt.Sprintf("%s:53", target))
		if err != nil {
			dnsFailures.Inc()
			return fmt.Errorf("could not complete dns exchange: %w", err)
		}

		if len(r.Answer) == 0 {
			return fmt.Errorf("dns message %d : %s", len(r.Answer), r.Answer)
		}

		dnsRttGauge.With(prometheus.Labels{
			"resolver": target,
		}).Set(rtt.Seconds())

	}
	return nil
}

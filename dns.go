package main

import (
	"fmt"
	"strconv"
	"time"

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
	dnsLatencyGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observer_dns_latency",
		Help: "Latency of DNS reequest.",
	}, []string{"resolver"})
	dnsAgeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observer_dns_age",
		Help: "Age of DNS healthcheck record.",
	}, []string{"resolver"})
)

func init() {
	prometheus.MustRegister(dnsLatencyGauge)
	prometheus.MustRegister(dnsAgeGauge)
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
			return err
		}

		if len(r.Answer) == 0 {
			return fmt.Errorf("dns message lenght %d", len(r.Answer))
		}

		dnsLatencyGauge.With(prometheus.Labels{
			"resolver": target,
		}).Set(rtt.Seconds())

		for _, ans := range r.Answer {
			if t, ok := ans.(*dns.TXT); ok {
				if len(t.Txt) > 0 {
					stamp, err := strconv.Atoi(t.Txt[0])
					if err != nil {
						return fmt.Errorf("failed to parse healthcheck record")
					}
					dnsAgeGauge.With(prometheus.Labels{
						"resolver": target,
					}).Set(float64(int(time.Now().Unix()) - stamp))
				}
			}
		}
	}
	return nil
}

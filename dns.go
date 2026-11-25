package main

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// dnsMetrics are the numbers of the DNS check over one address family.
type dnsMetrics struct {
	requests *prometheus.CounterVec
	failures *prometheus.CounterVec
	rtt      *prometheus.GaugeVec
}

// newDnsMetrics makes the metrics observer_dns<family>_... for family
// "4" or "6".
func newDnsMetrics(family string) dnsMetrics {
	return dnsMetrics{
		requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_dns" + family + "_requests",
			Help: "Total number of DNS requests sent over IPv" + family,
		}, []string{"resolver"}),
		failures: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "observer_dns" + family + "_failures",
			Help: "Total number of failed DNS requests over IPv" + family,
		}, []string{"resolver"}),
		rtt: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "observer_dns" + family + "_rtt",
			Help: "Latency of the last DNS request over IPv" + family + ".",
		}, []string{"resolver"}),
	}
}

var (
	dns4Metrics = newDnsMetrics("4")
	dns6Metrics = newDnsMetrics("6")
)

// dnsPort is the port the resolvers are asked on.
var dnsPort = "53"

// splitFamilies sorts resolver addresses into the IPv4 and the IPv6
// ones. A resolver is given by its address: which family a name would
// be asked over could not be told.
func splitFamilies(resolvers []string) (v4, v6 []string, err error) {
	for _, resolver := range resolvers {
		address, err := netip.ParseAddr(resolver)
		if err != nil {
			return nil, nil, fmt.Errorf("resolver %q is not an IP address", resolver)
		}
		if address.Unmap().Is4() {
			v4 = append(v4, resolver)
		} else {
			v6 = append(v6, resolver)
		}
	}
	return v4, v6, nil
}

// sampleDns asks every resolver of one address family, also after one
// that failed, and returns what failed.
func sampleDns(metrics dnsMetrics, targets []string, qname string) error {
	var errs []error
	for _, target := range targets {
		labels := prometheus.Labels{
			"resolver": target,
		}

		dnsFailures := metrics.failures.With(labels)

		metrics.requests.With(labels).Inc()

		dnsClient := dns.Client{}
		dnsMessage := dns.Msg{}
		dnsMessage.SetQuestion(qname, dns.TypeTXT)

		r, rtt, err := dnsClient.Exchange(&dnsMessage, net.JoinHostPort(target, dnsPort))
		if err != nil {
			dnsFailures.Inc()
			// a failed request has no latency: the last good one is not shown on
			metrics.rtt.Delete(labels)
			errs = append(errs, fmt.Errorf("could not complete dns exchange: %w", err))
			continue
		}

		// "no such name", a refusal or an answer without a record is a
		// failure too
		if r.Rcode != dns.RcodeSuccess || len(r.Answer) == 0 {
			dnsFailures.Inc()
			metrics.rtt.Delete(labels)
			errs = append(errs, fmt.Errorf("%s answered %s with %d records for %s",
				target, dns.RcodeToString[r.Rcode], len(r.Answer), qname))
			continue
		}

		metrics.rtt.With(labels).Set(rtt.Seconds())
	}
	return errors.Join(errs...)
}

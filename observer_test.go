package main

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

func TestSplitList(t *testing.T) {
	for list, want := range map[string][]string{
		"":                       nil,
		" ":                      nil,
		"192.0.2.1":              {"192.0.2.1"},
		"192.0.2.1,2001:db8::1":  {"192.0.2.1", "2001:db8::1"},
		"192.0.2.1, 192.0.2.2,,": {"192.0.2.1", "192.0.2.2"},
	} {
		if got := splitList(list); !reflect.DeepEqual(got, want) {
			t.Errorf("splitList(%q) = %q, want %q", list, got, want)
		}
	}
}

func TestSplitFamilies(t *testing.T) {
	v4, v6, err := splitFamilies([]string{"192.0.2.1", "2001:db8::1", "192.0.2.2", "::ffff:192.0.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"192.0.2.1", "192.0.2.2", "::ffff:192.0.2.3"}; !reflect.DeepEqual(v4, want) {
		t.Errorf("the IPv4 resolvers: %q, want %q", v4, want)
	}
	if want := []string{"2001:db8::1"}; !reflect.DeepEqual(v6, want) {
		t.Errorf("the IPv6 resolvers: %q, want %q", v6, want)
	}
	if v4, v6, err := splitFamilies(nil); err != nil || v4 != nil || v6 != nil {
		t.Errorf("no resolvers: %q, %q, %v", v4, v6, err)
	}
	// a name cannot be sorted into a family
	if _, _, err := splitFamilies([]string{"192.0.2.1", "resolver.example"}); err == nil {
		t.Error("a resolver given by name: no error")
	}
}

func TestAddressOf(t *testing.T) {
	v4, v6 := net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1")
	both := []net.IP{v6, v4, net.ParseIP("192.0.2.2")}
	if got := addressOf(both, false); !got.Equal(v4) {
		t.Errorf("the IPv4 address of a target with both: %v", got)
	}
	if got := addressOf(both, true); !got.Equal(v6) {
		t.Errorf("the IPv6 address of a target with both: %v", got)
	}
	// a target without an address of a family is not pinged over it
	if got := addressOf([]net.IP{v4}, true); got != nil {
		t.Errorf("the IPv6 address of an IPv4-only target: %v", got)
	}
	if got := addressOf([]net.IP{v6}, false); got != nil {
		t.Errorf("the IPv4 address of an IPv6-only target: %v", got)
	}
}

func TestAddressFromReply(t *testing.T) {
	reply := func(options ...dhcpv6.Option) *dhcpv6.Message {
		message, err := dhcpv6.NewMessage()
		if err != nil {
			t.Fatal(err)
		}
		message.MessageType = dhcpv6.MessageTypeReply
		for _, option := range options {
			message.AddOption(option)
		}
		return message
	}
	address := &dhcpv6.OptIAAddress{
		IPv6Addr:      net.ParseIP("2001:db8::1"),
		ValidLifetime: time.Hour,
	}

	got, err := addressFromReply(reply(&dhcpv6.OptIANA{
		Options: dhcpv6.IdentityOptions{Options: dhcpv6.Options{address}},
	}))
	if err != nil || !got.IPv6Addr.Equal(address.IPv6Addr) || got.ValidLifetime != time.Hour {
		t.Errorf("a reply with an address: got %v, %v", got, err)
	}

	// the two ways a server answers without an address: an error each,
	// not a crash
	if got, err := addressFromReply(reply()); err == nil {
		t.Errorf("a reply without an IA_NA: got %v and no error", got)
	}
	if got, err := addressFromReply(reply(&dhcpv6.OptIANA{})); err == nil {
		t.Errorf("a reply with an IA_NA without an address: got %v and no error", got)
	}
}

// resolver starts a name server on the loopback address given, on a
// port of its own which the questions are sent to from then on. It
// knows the TXT record ok.example. and no other name.
func resolver(t *testing.T, address string) {
	t.Helper()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(address, "0"))
	if err != nil {
		t.Fatal(err)
	}
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, question *dns.Msg) {
		answer := new(dns.Msg)
		answer.SetRcode(question, dns.RcodeNameError)
		_ = w.WriteMsg(answer)
	})
	mux.HandleFunc("ok.example.", func(w dns.ResponseWriter, question *dns.Msg) {
		answer := new(dns.Msg)
		answer.SetReply(question)
		answer.Answer = append(answer.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "ok.example.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
			Txt: []string{"ok"},
		})
		_ = w.WriteMsg(answer)
	})
	server := &dns.Server{PacketConn: conn, Handler: mux}
	go func() { _ = server.ActivateAndServe() }()

	_, port, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	before := dnsPort
	dnsPort = port
	t.Cleanup(func() {
		dnsPort = before
		_ = server.Shutdown()
	})
}

// metric returns the value of the metric with that name for that
// resolver, and whether there is one.
func metric(t *testing.T, name, resolver string) (float64, bool) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, label := range m.GetLabel() {
				if label.GetName() != "resolver" || label.GetValue() != resolver {
					continue
				}
				if m.GetCounter() != nil {
					return m.GetCounter().GetValue(), true
				}
				return m.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

func TestSampleDnsAnswered(t *testing.T) {
	resolver(t, "127.0.0.11")
	if err := sampleDns(dns4Metrics, []string{"127.0.0.11"}, "ok.example."); err != nil {
		t.Fatal(err)
	}
	if failures, _ := metric(t, "observer_dns4_failures", "127.0.0.11"); failures != 0 {
		t.Errorf("an answered question counted %v failures", failures)
	}
	if _, there := metric(t, "observer_dns4_rtt", "127.0.0.11"); !there {
		t.Error("an answered question left no latency")
	}
}

func TestSampleDnsNoSuchName(t *testing.T) {
	resolver(t, "127.0.0.12")
	// answered first, so that there is a latency to take away
	if err := sampleDns(dns4Metrics, []string{"127.0.0.12"}, "ok.example."); err != nil {
		t.Fatal(err)
	}
	err := sampleDns(dns4Metrics, []string{"127.0.0.12"}, "missing.example.")
	if err == nil || !strings.Contains(err.Error(), "NXDOMAIN") {
		t.Errorf("a name that does not exist: error %v", err)
	}
	if failures, _ := metric(t, "observer_dns4_failures", "127.0.0.12"); failures != 1 {
		t.Errorf("a name that does not exist counted %v failures, want 1", failures)
	}
	if latency, there := metric(t, "observer_dns4_rtt", "127.0.0.12"); there {
		t.Errorf("the failed question still shows a latency: %v", latency)
	}
}

func TestSampleDnsAsksEveryResolver(t *testing.T) {
	resolver(t, "127.0.0.13")
	// nothing listens on 127.0.0.14: the first resolver fails
	err := sampleDns(dns4Metrics, []string{"127.0.0.14", "127.0.0.13"}, "ok.example.")
	if err == nil {
		t.Error("a resolver that does not answer: no error")
	}
	if failures, _ := metric(t, "observer_dns4_failures", "127.0.0.14"); failures != 1 {
		t.Errorf("the silent resolver counted %v failures, want 1", failures)
	}
	if requests, _ := metric(t, "observer_dns4_requests", "127.0.0.13"); requests != 1 {
		t.Errorf("the resolver after the silent one was asked %v times, want 1", requests)
	}
	if failures, _ := metric(t, "observer_dns4_failures", "127.0.0.13"); failures != 0 {
		t.Errorf("the resolver after the silent one counted %v failures", failures)
	}
}

func TestSampleDnsIPv6Resolver(t *testing.T) {
	// whether or not anything answers there: the address must be one
	// that can be asked
	err := sampleDns(dns6Metrics, []string{"::1"}, "ok.example.")
	if err != nil && strings.Contains(err.Error(), "too many colons") {
		t.Errorf("a resolver given as an IPv6 address: %v", err)
	}
}

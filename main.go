package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vishvananda/netlink"
)

var (
	iface       string
	icmpTargets string
	icmpCount   int
	interval    time.Duration
	verbose     bool
	disable4    bool
	disable6    bool
	// one switch for every check
	disableDhcp4 bool
	disableDhcp6 bool
	disableDns4  bool
	disableDns6  bool
	disableIcmp4 bool
	disableIcmp6 bool
	dnsQname     string
	dnsTargets   string
	hostPort     int
)

var Version = "0-development"

func main() {
	slog.Info("running", "version", Version)
	flag.StringVar(&iface, "interface", "", "Interface to operate on")
	flag.StringVar(&icmpTargets, "icmp-targets", "", "Comma sepperated list with ICMP targets")
	flag.IntVar(&icmpCount, "icmp-count", 3, "ICMP count")
	flag.DurationVar(&interval, "interval", time.Second*5, "Interval in seconds to collect prometheus metrics")
	flag.BoolVar(&verbose, "verbose", false, "Toggles verbose logging")
	flag.BoolVar(&disable4, "disable4", false, "Disable all IPv4 client behaviour - DHCPv4, DNS and ICMP over IPv4")
	flag.BoolVar(&disable6, "disable6", false, "Disable all IPv6 client behaviour - DHCPv6, DNS and ICMP over IPv6")
	flag.BoolVar(&disableDhcp4, "disable-dhcp4", false, "Disable the DHCPv4 check")
	flag.BoolVar(&disableDhcp6, "disable-dhcp6", false, "Disable the DHCPv6 check")
	flag.BoolVar(&disableDns4, "disable-dns4", false, "Disable the DNS check of the IPv4 resolvers")
	flag.BoolVar(&disableDns6, "disable-dns6", false, "Disable the DNS check of the IPv6 resolvers")
	flag.BoolVar(&disableIcmp4, "disable-icmp4", false, "Disable ICMP over IPv4")
	flag.BoolVar(&disableIcmp6, "disable-icmp6", false, "Disable ICMP over IPv6")
	flag.StringVar(&dnsQname, "qname", "healthcheck.event.dreamhack.se.", "DNS healthcheck qname to probe.")
	flag.StringVar(&dnsTargets, "dns", "", "DNS servers to probe, IPv4 and IPv6 addresses, separate by comma.")
	flag.IntVar(&hostPort, "host-port", 9023, "The port to listen on. Default: 9023")
	flag.Parse()

	if iface == "" {
		slog.Error("interface must be specified")
		flag.Usage()
		os.Exit(1)
	}

	// -disable4 and -disable6 are every check of that family
	if disable4 {
		disableDhcp4, disableDns4, disableIcmp4 = true, true, true
	}
	if disable6 {
		disableDhcp6, disableDns6, disableIcmp6 = true, true, true
	}

	if disableDhcp4 && disableDhcp6 && disableDns4 && disableDns6 && disableIcmp4 && disableIcmp6 {
		slog.Error("every check is disabled, needs at least one")
		flag.Usage()
		os.Exit(1)
	}

	link, err := netlink.LinkByName(iface)
	if err != nil {
		log.Fatalln(err)
	}

	slog.Info("starting observer", "iface", iface, "icmp_targets", icmpTargets, "icmp_count", icmpCount, "interval", interval, "dns_qname", dnsQname, "dns_target", dnsTargets,
		"disable_dhcp4", disableDhcp4, "disable_dhcp6", disableDhcp6, "disable_dns4", disableDns4, "disable_dns6", disableDns6, "disable_icmp4", disableIcmp4, "disable_icmp6", disableIcmp6)

	dns4TargetsList, dns6TargetsList, err := splitFamilies(splitList(dnsTargets))
	if err != nil {
		slog.Error("-dns", "err", err)
		os.Exit(1)
	}
	icmpTargetsList := splitList(icmpTargets)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for {

			if !disableDhcp4 {
				if err := observeIPv4(link); err != nil {
					slog.Warn("issue with ipv4", "err", err)
				}
			}

			if !disableDhcp6 {
				if err := observeIPv6(link); err != nil {
					slog.Warn("issue with ipv6", "err", err)
				}
			}

			if !disableDns4 {
				if err := sampleDns(dns4Metrics, dns4TargetsList, dnsQname); err != nil {
					slog.Warn("issue with DNS sampling over IPv4", "err", err)
				}
			}

			if !disableDns6 {
				if err := sampleDns(dns6Metrics, dns6TargetsList, dnsQname); err != nil {
					slog.Warn("issue with DNS sampling over IPv6", "err", err)
				}
			}

			if err := sampleIcmp(icmpTargetsList, icmpCount, !disableIcmp4, !disableIcmp6); err != nil {
				slog.Warn("issue sampling icmp", "err", err)
			}

			time.Sleep(interval)
		}
	}()

	hostAddress := fmt.Sprintf(":%d", hostPort)
	slog.Info("starting observer", "host_address", hostAddress)
	err = http.ListenAndServe(hostAddress, nil)
	if err != nil {
		slog.Error("http server", "err", err)
		os.Exit(1)
	}
}

// splitList returns the entries of a comma separated list. An empty
// list has none: a check that was given nothing to ask is not made.
func splitList(list string) []string {
	var entries []string
	for _, entry := range strings.Split(list, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func observeIPv4(link netlink.Link) error {
	ip, prefix, err := sampleDhcp(interval, verbose)
	if err != nil {
		return fmt.Errorf("DHCPv4 sampling failed: %w", err)
	}

	addr, err := netlink.ParseAddr(ip.String() + "/" + strconv.Itoa(prefix))
	if err != nil {
		return fmt.Errorf("parsing IPv4 address failed: %w", err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("getting address list: %w", err)
	}

	for _, address := range addrs {
		if address.IP.Equal(addr.IP) {
			return nil
		}
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		slog.Warn("could not add IPv4 address", "addr", addr, "err", err)
		return nil
	}
	defer func() {
		if err := netlink.AddrDel(link, addr); err != nil {
			slog.Warn("could not delete IPv4 address", "addr", addr, "err", err)
		}
	}()

	return nil
}

func observeIPv6(link netlink.Link) error {
	ip6, prefix6, err := sampleDhcp6(interval, verbose)
	if err != nil {
		return fmt.Errorf("DHCPv6 sampling failed: %w", err)
	}

	addr6, err := netlink.ParseAddr(ip6.String() + "/" + strconv.Itoa(prefix6))
	if err != nil {
		return fmt.Errorf("parsing IPv6 address failed: %w", err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return fmt.Errorf("getting address list: %w", err)
	}

	for _, address := range addrs {
		if address.IP.Equal(addr6.IP) {
			return nil
		}
	}

	if err := netlink.AddrAdd(link, addr6); err != nil {
		slog.Warn("could not add IPv6 address", "addr6", addr6, "err", err)
		return nil
	}
	defer func() {
		if err := netlink.AddrDel(link, addr6); err != nil {
			slog.Warn("could not delete IPv6 address", "addr6", addr6, "err", err)
		}
	}()

	return nil
}

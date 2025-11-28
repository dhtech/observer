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

	"github.com/insomniacslk/dhcp/dhcpv4/client4"
	"github.com/insomniacslk/dhcp/dhcpv6/client6"
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
	dnsQname    string
	dnsTargets  string
	hostPort    int
)

var Version = "0-development"

func main() {
	slog.Info("running", "version", Version)
	flag.StringVar(&iface, "interface", "", "Interface to operate on")
	flag.StringVar(&icmpTargets, "icmp-targets", "", "Comma sepperated list with ICMP targets")
	flag.IntVar(&icmpCount, "icmp-count", 3, "ICMP count")
	flag.DurationVar(&interval, "interval", time.Second*5, "Interval in seconds to collect prometheus metrics")
	flag.BoolVar(&verbose, "verbose", false, "Toggles verbose logging")
	flag.BoolVar(&disable4, "disable4", false, "Disable all IPv4 client behaviour - DHCPv4 and ICMPv4")
	flag.BoolVar(&disable6, "disable6", false, "Disable all IPv6 client behaviour - DHCPv6 and ICMPv6")
	flag.StringVar(&dnsQname, "qname", "healthcheck.event.dreamhack.se.", "DNS healthcheck qname to probe.")
	flag.StringVar(&dnsTargets, "dns", "", "DNS servers to probe, separate by comma.")
	flag.IntVar(&hostPort, "host-port", 9023, "The port to listen on. Default: 9023")
	flag.Parse()

	if iface == "" {
		slog.Error("interface must be specified")
		flag.Usage()
		os.Exit(1)
	}

	if disable4 && disable6 {
		slog.Error("both disable4 and disable6 flag set, needs at least one")
		flag.Usage()
		os.Exit(1)
	}

	link, err := netlink.LinkByName(iface)
	if err != nil {
		log.Fatalln(err)
	}

	slog.Info("starting observer", "iface", iface, "icmp_targets", icmpTargets, "icmp_count", icmpCount, "interval", interval, "disable4", disable4, "disable6", disable6, "dns_qname", dnsQname, "dns_target", dnsTargets)

	dnsTargetsList := strings.Split(dnsTargets, ",")
	icmpTargetsList := strings.Split(icmpTargets, ",")

	ipv4DhcpClient := client4.Client{
		ReadTimeout:  interval,
		WriteTimeout: interval,
	}

	ipv6DhcpClient := client6.Client{
		ReadTimeout:  interval,
		WriteTimeout: interval,
	}

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for {

			if !disable4 {
				if err := observeIPv4(link, ipv4DhcpClient); err != nil {
					slog.Warn("issue with ipv4", "err", err)
				}
			}

			if !disable6 {
				if err := observeIPv6(link, ipv6DhcpClient); err != nil {
					slog.Warn("issue with ipv6", "err", err)
				}
			}

			if err := sampleDns(dnsTargetsList, dnsQname); err != nil {
				slog.Warn("issue with DNS sampling", "err", err)
			}

			if err := sampleIcmp(icmpTargetsList, icmpCount); err != nil {
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

func observeIPv4(link netlink.Link, client client4.Client) error {
	ip, prefix, err := sampleDhcp(client, verbose)
	if err != nil {
		return fmt.Errorf("DHCPv4 sampling failed: %w", err)
	}

	addr, err := netlink.ParseAddr(ip.String() + "/" + strconv.Itoa(prefix))
	if err != nil {
		return fmt.Errorf("parsing IPv4 address failed: %w", err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
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

func observeIPv6(link netlink.Link, client client6.Client) error {
	ip6, prefix6, err := sampleDhcp6(client, verbose)
	if err != nil {
		return fmt.Errorf("DHCPv6 sampling failed: %w", err)
	}

	addr6, err := netlink.ParseAddr(ip6.String() + "/" + strconv.Itoa(prefix6))
	if err != nil {
		return fmt.Errorf("parsing IPv6 address failed: %w", err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
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

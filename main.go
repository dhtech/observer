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

	if disable4 && disable6 {
		slog.Error("both disable4 and disable6 flag set, needs at least one")
		flag.Usage()
		os.Exit(1)
	}

	slog.Info("starting observer", "iface", iface, "icmp_targets", icmpTargets, "icmp_count", icmpCount, "interval", interval, "disable4", disable4, "disable6", disable6, "dns_qname", dnsQname, "dns_target", dnsTargets)

	dnsTargetsList := strings.Split(dnsTargets, ",")
	icmpTargetsList := strings.Split(icmpTargets, ",")
	http.Handle("/metrics", promhttp.Handler())

	link, err := netlink.LinkByName(iface)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			yourIPAddr, prefixBits, err := sampleDhcp(iface, verbose)
			if err != nil {
				slog.Warn("sampling dhcp", "your_ip_addr", yourIPAddr, "prefix_bits", prefixBits, "err", err)
			}

			err = sampleDns(dnsTargetsList, dnsQname)
			if err != nil {
				slog.Warn("sampling DNS", "err", err)
			}

			var addr *netlink.Addr
			var addr6 *netlink.Addr
			if !disable4 {
				yourIPAddr, prefixBits, err := sampleDhcp(iface, verbose)
				if err != nil {
					slog.Warn("sampling dhcp v4", "iface", iface, "err", err)
				}

				addr, err = netlink.ParseAddr(yourIPAddr.String() + "/" + strconv.Itoa(prefixBits))
				if err != nil {
					slog.Error("parsing address", "err", err)
					os.Exit(1)
				}

				err = netlink.AddrAdd(link, addr)
				if err != nil {
					slog.Warn("adding netlink", "link", link, "addr", addr, "err", err)
				}
			}

			if !disable6 {
				yourIP6Addr, prefix6Bits, err := sampleDhcp6(iface, verbose)
				if err != nil {
					slog.Warn("sampling dhcp v6", "iface", iface, "err", err)
				}

				addr6, err = netlink.ParseAddr(yourIP6Addr.String() + "/" + strconv.Itoa(prefix6Bits))
				if err != nil {
					slog.Error("adding link", "addr6", addr6, "err", err)
					os.Exit(1)
				}

				err = netlink.AddrAdd(link, addr6)
				if err != nil {
					slog.Warn("could not add netlink address", "link", link, "addr6", addr6, "err", err)
				}
			}

			if icmpTargets != "" && len(icmpTargetsList) > 0 {
				sampleIcmp(icmpTargetsList, icmpCount)
			}

			if !disable4 {
				err := netlink.AddrDel(link, addr)
				if err != nil {
					slog.Warn("deleting v4 netlink", "link", link, "addr", addr, "err", err)
				}
			}

			if !disable6 {
				err := netlink.AddrDel(link, addr6)
				if err != nil {
					slog.Warn("deleting v6 netlink", "link", link, "addr", addr, "err", err)
				}
			}

			time.Sleep(interval)
		}
	}()

	hostAddress := fmt.Sprintf(":%d", hostPort)
	slog.Info("starting observer", "host_address", hostAddress)
	err = http.ListenAndServe(hostAddress, nil)
	if err != nil {
		panic(err)
	}
}

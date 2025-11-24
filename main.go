package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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

func main() {
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
		log.Fatalf("You have to leave at least one address family enabled! Exiting")
	}

	fmt.Println("Starting observer.")
	dnsTargetsList := strings.Split(dnsTargets, ",")
	icmpTargetsList := strings.Split(icmpTargets, ",")
	http.Handle("/metrics", promhttp.Handler())

	link, err := netlink.LinkByName(iface)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			sampleDhcp(iface, verbose)
			if len(dnsTargetsList) > 0 {
				sampleDns(dnsTargetsList, dnsQname)
			}
			var addr *netlink.Addr
			var addr6 *netlink.Addr
			if !disable4 {
				err, yourIPAddr, prefixBits := sampleDhcp(iface, verbose)
				if err != nil {
					log.Println("DHCPv4 check reported an error: ", err)
				}
				addr, err = netlink.ParseAddr(yourIPAddr.String() + "/" + strconv.Itoa(prefixBits))
				if err != nil {
					log.Fatalln(err)
				}
				netlink.AddrAdd(link, addr)

			}
			if !disable6 {
				err, yourIP6Addr, prefix6Bits := sampleDhcp6(iface, verbose)
				if err != nil {
					log.Println("DHCPv6 check reported an error: ", err)
				}
				addr6, err = netlink.ParseAddr(yourIP6Addr.String() + "/" + strconv.Itoa(prefix6Bits))
				if err != nil {
					log.Fatalln(err)
				}
				netlink.AddrAdd(link, addr6)
			}

			if icmpTargets != "" && len(icmpTargetsList) > 0 {
				sampleIcmp(icmpTargetsList, icmpCount)
			}

			if !disable4 {
				netlink.AddrDel(link, addr)
			}
			if !disable6 {
				netlink.AddrDel(link, addr6)
			}
			time.Sleep(interval)
		}
	}()

	hostAddress := fmt.Sprintf(":%d", hostPort)
	fmt.Println(hostAddress)
	err = http.ListenAndServe(hostAddress, nil)
	if err != nil {
		panic(err)
	}
}

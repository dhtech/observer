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
	iface       = flag.String("interface", "", "Interface to operate on")
	icmpTargets = flag.String("icmp", "", "ICMP target.")
	icmpCount   = flag.Int("icmp-count", 3, "ICMP count.")
	interval    = flag.Duration("interval", time.Second*5, "Collection interval")
	verbose     = flag.Bool("verbose", false, "Verbose")
	disable4    = flag.Bool("disable4", false, "Disable all IPv4 client behavior - DHCPv4 and ICMPv4")
	disable6    = flag.Bool("disable6", false, "Disable all IPv6 client behavior - DHCPv6 and ICMPv6")
)

func main() {
	flag.Parse()

	if *disable4 && *disable6 {
		log.Fatalf("You have to leave at least one address family enabled! Exiting")
	}

	fmt.Println("Starting observer.")
	icmpTargetsList := strings.Split(*icmpTargets, ",")
	http.Handle("/metrics", promhttp.Handler())

	link, err := netlink.LinkByName(*iface)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			var addr *netlink.Addr
			var addr6 *netlink.Addr
			if !*disable4 {
				err, yourIPAddr, prefixBits := sampleDhcp(*iface, *verbose)
				if err != nil {
					log.Println("DHCPv4 check reported an error: ", err)
				}
				//log.Printf("Got IP %s/%d", yourIPAddr.String(), prefixBits)
				addr, err = netlink.ParseAddr(yourIPAddr.String() + "/" + strconv.Itoa(prefixBits))
				if err != nil {
					log.Fatalln(err)
				}
				netlink.AddrAdd(link, addr)

			}
			if !*disable6 {
				err, yourIP6Addr, prefix6Bits := sampleDhcp6(*iface, *verbose)
				if err != nil {
					log.Println("DHCPv4 check reported an error: ", err)
				}
				//log.Printf("Got IPv6 %s/%d", yourIP6Addr.String(), prefix6Bits)
				addr6, err = netlink.ParseAddr(yourIP6Addr.String() + "/" + strconv.Itoa(prefix6Bits))
				if err != nil {
					log.Fatalln(err)
				}
				netlink.AddrAdd(link, addr6)
			}

			if *icmpTargets != "" && len(icmpTargetsList) > 0 {
				sampleIcmp(icmpTargetsList, *icmpCount)
			}

			if !*disable4 {
				netlink.AddrDel(link, addr)
			}
			if !*disable6 {
				netlink.AddrDel(link, addr6)
			}
			time.Sleep(*interval)
		}
	}()

	err = http.ListenAndServe(":9023", nil)
	if err != nil {
		panic(err)
	}
}

# observer

A lightweight network observer written in Go that monitors network interfaces, performs ICMP and DNS health checks, and exposes Prometheus metrics.

## features

- Periodically samples DHCPv4 and DHCPv6 addresses on a specified interface.
- Performs ICMP pings to one or more target hosts.
- Performs DNS health checks for specified DNS servers and query names.
- Dynamically adds and removes IP addresses on the interface during checks.
- Exposes Prometheus metrics via an HTTP endpoint.
- Supports toggling IPv4 or IPv6 monitoring independently.
- Verbose logging for debugging and operational insight.

## installation

```bash
git clone git@github.com:dhtech/observer.git
cd observer
go build .
./observer
```

## configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-interface` | `""` | Network interface to operate on |
| `-icmp-targets` | `""` | Comma-separated list of ICMP targets; a name is pinged over IPv4 and over IPv6, each if it has such an address |
| `-icmp-count` | `3` | Number of ICMP packets to send per target |
| `-interval` | `5s` | Interval for collecting Prometheus metrics |
| `-verbose` | `false` | Enable verbose logging |
| `-disable4` | `false` | Disable all IPv4 client behavior (DHCPv4, DNS and ICMP over IPv4) |
| `-disable6` | `false` | Disable all IPv6 client behavior (DHCPv6, DNS and ICMP over IPv6) |
| `-disable-dhcp4` | `false` | Disable the DHCPv4 check |
| `-disable-dhcp6` | `false` | Disable the DHCPv6 check |
| `-disable-dns4` | `false` | Disable the DNS check of the IPv4 resolvers |
| `-disable-dns6` | `false` | Disable the DNS check of the IPv6 resolvers |
| `-disable-icmp4` | `false` | Disable ICMP over IPv4 |
| `-disable-icmp6` | `false` | Disable ICMP over IPv6 |
| `-qname` | `healthcheck.event.dreamhack.se.` | DNS health check query name |
| `-dns` | `""` | Comma-separated DNS servers to probe, by address: the IPv4 ones are the DNS check over IPv4, the IPv6 ones the check over IPv6 |
| `-host-port` | `9023` | HTTP port to serve Prometheus metrics |

example:
```bash
./observer \
  -interface eth0 \
  -icmp-targets 8.8.8.8,1.1.1.1 \
  -icmp-count 5 \
  -dns 8.8.8.8,1.1.1.1 \
  -interval 10s \
  -verbose
```

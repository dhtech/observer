#!/bin/sh
# Tries the package build.sh left in dist/, as a machine would have it.
# Runs INSIDE a NEW debian:13 container that has nothing of the build:
#
#   docker run --rm --cap-add NET_ADMIN --cap-add NET_RAW \
#       -v "$PWD:/src:ro" -w /src debian:13 sh .github/try.sh 1.2.3
#
# (NET_ADMIN and NET_RAW: what the unit lets the program keep, and a
# container does not have by itself.)
#
# The package installs with apt; the program says the version; the unit
# is there, systemd finds no fault in it, and it is neither enabled nor
# started; the program, started with the privileges the unit leaves it
# and no others, asks for an address and serves its numbers; and the
# package can be removed again.
#
# NOT tried here: the unit started BY systemd, and the package's
# restart of a running observer at an upgrade - a container like this
# one has no systemd running in it. Nor an address really given: there
# is no DHCP server here that answers.
set -eu

version=${1:?the version, like 1.2.3}
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -y -q --no-install-recommends systemd \
    ./dist/observer_*_"$(dpkg --print-architecture)".deb
dpkg -s observer | grep -E '^(Status|Version):'
# from here on every step is shown: one that fails is the last one seen
set -x
test "$(dpkg-query -W -f='${Version}' observer)" = "$version"
# without an interface it says its version, how it is called, and ends
said=$(observer 2>&1 || true)
echo "$said" | head -3
echo "$said" | grep -q "version=v$version"

unit=/usr/lib/systemd/system/observer.service
test -f "$unit"
grep -qxF 'ExecStart=/usr/sbin/observer $OBSERVER_OPTS' "$unit"
# systemd reads the unit and finds no fault in it
systemd-analyze verify "$unit"
# (a "slim" image leaves /usr/share/doc out of every package: not there)
test -s /usr/share/doc/observer/THIRD-PARTY-LICENSES
# nothing was enabled
test ! -e /etc/systemd/system/multi-user.target.wants/observer.service

# the unit's way, by hand: root with the three capabilities the unit
# leaves it, on this machine's network interface
grep -qxF 'CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE' "$unit"
iface=$(ls /sys/class/net | grep -vx lo | head -1)
test -n "$iface"
setpriv --bounding-set=-all,+net_admin,+net_raw,+net_bind_service \
    --inh-caps=-all --no-new-privs \
    observer -interface "$iface" -interval 1s > /tmp/observer.log 2>&1 &
pid=$!
# when it fails from here on, show what the program said
trap 'rc=$?; kill "$pid" 2>/dev/null || true; [ "$rc" = 0 ] || { echo "--- the program said:"; cat /tmp/observer.log; }' EXIT
metrics() {
    bash -c 'exec 3<>/dev/tcp/127.0.0.1/9023 &&
        printf "GET /metrics HTTP/1.0\r\n\r\n" >&3 && cat <&3' 2>/dev/null
}
# waited for, not guessed at: at most 30 seconds until it has asked for
# an address over IPv4 and over IPv6
n=0
until metrics | grep -Eq '^observer_dhcp_requests [1-9]' &&
    metrics | grep -Eq '^observer_dhcp6_requests [1-9]'; do
    n=$((n + 1))
    [ "$n" -le 150 ] || { echo "it did not ask for an address in 30 s"; exit 1; }
    sleep 0.2
done
metrics | grep -E '^observer_dhcp6?_(requests|failures)'
# it had those three capabilities and no other (bits 10, 12 and 13) ...
grep -E '^Cap(Prm|Eff|Bnd):[[:space:]]+0000000000003400$' "/proc/$pid/status" | wc -l | grep -qx 3
# ... and they were enough: nothing it tried was refused
if grep -Ei 'not permitted|permission denied' /tmp/observer.log; then
    echo "something was refused with the unit's privileges"
    exit 1
fi
kill "$pid"
wait "$pid" || true

# removed: the program goes, somebody's settings stay; purged: they go too
echo 'OBSERVER_OPTS="-interface eth0"' > /etc/default/observer
dpkg -r observer
test ! -e /usr/sbin/observer
test ! -e "$unit"
test -e /etc/default/observer
dpkg -P observer
test ! -e /etc/default/observer
echo "package tried: ok"

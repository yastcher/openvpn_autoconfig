#!/bin/sh
set -e

# Ensure TUN device exists
mkdir -p /dev/net
[ ! -c /dev/net/tun ] && mknod /dev/net/tun c 10 200

# Default command: start OpenVPN server
if [ "${1:-serve}" = "serve" ]; then
    # NAT for VPN clients
    iptables -t nat -A POSTROUTING -s 192.168.255.0/24 -j MASQUERADE
    iptables -A FORWARD -i tun0 -j ACCEPT
    iptables -A FORWARD -o tun0 -j ACCEPT

    if [ -n "${VPN_IPV6_SUBNET:-}" ]; then
        ip6tables -t nat -A POSTROUTING -s "$VPN_IPV6_SUBNET" -j MASQUERADE
        ip6tables -A FORWARD -i tun0 -j ACCEPT
        ip6tables -A FORWARD -o tun0 -j ACCEPT
    fi

    exec openvpn --config /etc/openvpn/openvpn.conf
fi

# Otherwise pass through (e.g. "vpn setup", "vpn create", "sh")
exec "$@"

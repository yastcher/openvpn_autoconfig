FROM alpine:3.20
RUN apk add --no-cache openvpn easy-rsa iptables && \
    ln -s /usr/share/easy-rsa/easyrsa /usr/local/bin/easyrsa
COPY vpn.sh /usr/local/bin/vpn
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /usr/local/bin/vpn /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["serve"]

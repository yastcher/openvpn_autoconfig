FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod main.go ./
RUN go build -ldflags="-s -w" -o /vpn .

FROM alpine:3.20
RUN apk add --no-cache openvpn easy-rsa iptables && \
    ln -s /usr/share/easy-rsa/easyrsa /usr/local/bin/easyrsa
COPY --from=builder /vpn /usr/local/bin/vpn
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["serve"]

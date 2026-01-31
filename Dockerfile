FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod main.go ./
RUN go build -ldflags="-s -w" -o /vpn .

FROM kylemanna/openvpn
COPY --from=builder /vpn /usr/local/bin/vpn

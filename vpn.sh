#!/bin/sh
set -e

OVPN_DIR="/etc/openvpn"
PKI_DIR="/etc/openvpn/pki"
CLIENTS_DIR="/clients"

# ──────────────────────────────────────────────
#  helpers
# ──────────────────────────────────────────────

fatal() {
    printf "❌ %s\n" "$1" >&2
    exit 1
}

step() {
    printf "==> [%s] %s\n" "$1" "$2"
}

set_easyrsa_env() {
    export EASYRSA_PKI="$PKI_DIR"
    export EASYRSA_BATCH=1
    export EASYRSA_ALGO=ec
    export EASYRSA_CURVE=prime256v1
}

check_pki() {
    [ -d "$PKI_DIR" ] || fatal "Server not initialized. First run: docker compose run --rm openvpn vpn setup"
}

load_vpn_env() {
    VPN_IP="${VPN_SERVER_IP:-}"
    VPN_PORT_VAL="${VPN_PORT:-}"

    # Fallback: read from saved file
    if [ -z "$VPN_IP" ] || [ -z "$VPN_PORT_VAL" ]; then
        if [ -f "$OVPN_DIR/vpn.env" ]; then
            . "$OVPN_DIR/vpn.env"
            [ -z "$VPN_IP" ] && VPN_IP="$VPN_SERVER_IP"
            [ -z "$VPN_PORT_VAL" ] && VPN_PORT_VAL="$VPN_PORT"
        fi
    fi

    [ -z "$VPN_IP" ] && fatal "VPN_SERVER_IP not set"
    VPN_PORT_VAL="${VPN_PORT_VAL:-443}"
}

# Extract only the PEM block from EasyRSA cert files
extract_pem() {
    sed -n "/-----BEGIN $1-----/,/-----END $1-----/p" "$2"
}

# ──────────────────────────────────────────────
#  setup
# ──────────────────────────────────────────────

cmd_setup() {
    ip="${VPN_SERVER_IP:-}"
    [ -z "$ip" ] || [ "$ip" = "YOUR_SERVER_IP" ] && fatal "VPN_SERVER_IP not set. Check .env"

    port="${VPN_PORT:-443}"

    [ -d "$PKI_DIR" ] && fatal "PKI already initialized. To reset: delete ./data/ and run again."

    printf "\n"
    printf "╔══════════════════════════════════════╗\n"
    printf "║  OpenVPN Setup                       ║\n"
    printf "║  IP:   %-30s║\n" "$ip"
    printf "║  Port: %-30s║\n" "${port}/tcp"
    printf "╚══════════════════════════════════════╝\n"
    printf "\n"

    set_easyrsa_env

    # 1. Initialize PKI
    step "1/5" "Initializing PKI"
    easyrsa init-pki

    # 2. Build CA (ECDSA P-256, no password)
    step "2/5" "Building CA (ECDSA P-256)"
    EASYRSA_REQ_CN="OpenVPN-CA" easyrsa build-ca nopass

    # 3. Generate server certificate
    step "3/5" "Generating server certificate"
    easyrsa build-server-full server nopass

    # 4. Generate tls-crypt key + initial CRL
    step "4/5" "Generating tls-crypt key and CRL"
    openvpn --genkey --secret "$PKI_DIR/ta.key"
    easyrsa gen-crl
    chmod 644 "$PKI_DIR/crl.pem"

    # 5. Write server config
    step "5/5" "Writing server config"

    cat > "$OVPN_DIR/openvpn.conf" <<EOF
port 443
proto tcp
dev tun
tcp-nodelay
sndbuf 0
rcvbuf 0
ca ${PKI_DIR}/ca.crt
cert ${PKI_DIR}/issued/server.crt
key ${PKI_DIR}/private/server.key
dh none
topology subnet
server 192.168.255.0 255.255.255.0
ifconfig-pool-persist /etc/openvpn/ipp.txt
push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 1.0.0.1"
push "block-outside-dns"
push "sndbuf 0"
push "rcvbuf 0"
keepalive 10 120
cipher AES-256-GCM
auth SHA256
tls-crypt ${PKI_DIR}/ta.key
tls-version-min 1.2
tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384
crl-verify ${PKI_DIR}/crl.pem
duplicate-cn
persist-key
persist-tun
log /dev/null
status /dev/null
verb 0
EOF

    # Save connection info for client generation
    printf "VPN_SERVER_IP=%s\nVPN_PORT=%s\n" "$ip" "$port" > "$OVPN_DIR/vpn.env"

    # Save EasyRSA vars for future use
    printf "set_var EASYRSA_ALGO     ec\nset_var EASYRSA_CURVE    prime256v1\n" > "$PKI_DIR/vars"

    printf "\n✅ Server initialized!\n\n"
    printf "  docker compose up -d                              # start server\n"
    printf "  docker compose exec openvpn vpn create phone      # create client\n\n"
}

# ──────────────────────────────────────────────
#  create
# ──────────────────────────────────────────────

cmd_create() {
    name="$1"
    [ -z "$name" ] && fatal "Usage: vpn create <name>"

    check_pki

    out_path="$CLIENTS_DIR/${name}.ovpn"
    [ -f "$out_path" ] && fatal "File $out_path already exists. Delete it or choose another name."

    mkdir -p "$CLIENTS_DIR"
    load_vpn_env

    printf "==> Creating client: %s\n" "$name"
    set_easyrsa_env
    easyrsa build-client-full "$name" nopass

    printf "==> Assembling .ovpn...\n"

    ca_cert=$(cat "$PKI_DIR/ca.crt")
    client_cert=$(extract_pem "CERTIFICATE" "$PKI_DIR/issued/${name}.crt")
    client_key=$(cat "$PKI_DIR/private/${name}.key")
    ta_key=$(cat "$PKI_DIR/ta.key")

    cat > "$out_path" <<EOF
client
dev tun
proto tcp
remote ${VPN_IP} ${VPN_PORT_VAL}
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
verify-x509-name server name
auth SHA256
auth-nocache
cipher AES-256-GCM
tls-client
tls-version-min 1.2
tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384
ignore-unknown-option block-outside-dns
setenv opt block-outside-dns
verb 3
<ca>
${ca_cert}
</ca>
<cert>
${client_cert}
</cert>
<key>
${client_key}
</key>
<tls-crypt>
${ta_key}
</tls-crypt>
EOF

    chmod 600 "$out_path"

    printf "\n✅ %s\n" "$out_path"
    printf "   Copy to device → import into OpenVPN Connect.\n"
    printf "   File contains all keys — store like a password!\n\n"
}

# ──────────────────────────────────────────────
#  revoke
# ──────────────────────────────────────────────

cmd_revoke() {
    name="$1"
    [ -z "$name" ] && fatal "Usage: vpn revoke <name>"

    check_pki

    cert_path="$PKI_DIR/issued/${name}.crt"
    [ -f "$cert_path" ] || fatal "Client '$name' not found."

    printf "==> Revoking client: %s\n" "$name"
    set_easyrsa_env
    easyrsa revoke "$name"
    easyrsa gen-crl
    chmod 644 "$PKI_DIR/crl.pem"

    rm -f "$CLIENTS_DIR/${name}.ovpn"

    printf "\n✅ Client %s revoked. Its .ovpn no longer works.\n\n" "$name"
}

# ──────────────────────────────────────────────
#  list
# ──────────────────────────────────────────────

cmd_list() {
    check_pki

    issued_dir="$PKI_DIR/issued"
    index_file="$PKI_DIR/index.txt"

    printf "Clients:\n"
    count=0

    for cert in "$issued_dir"/*.crt; do
        [ -f "$cert" ] || continue
        name=$(basename "$cert" .crt)
        [ "$name" = "server" ] && continue

        count=$((count + 1))

        if [ -f "$index_file" ] && grep -q "^R.*\/CN=${name}$" "$index_file" 2>/dev/null; then
            status="⊘ revoked"
        elif [ -f "$CLIENTS_DIR/${name}.ovpn" ]; then
            status="✓ $CLIENTS_DIR/${name}.ovpn"
        else
            status="✗ ovpn not exported"
        fi

        printf "  • %-20s %s\n" "$name" "$status"
    done

    [ "$count" -eq 0 ] && printf "  (empty — create client: vpn create <name>)\n"
    printf "\n"
}

# ──────────────────────────────────────────────
#  main
# ──────────────────────────────────────────────

print_usage() {
    cat <<'EOF'
vpn — OpenVPN server management

Commands:
  setup           Initialize server (one time)
  create <name>   Create client → /clients/<name>.ovpn
  revoke <name>   Revoke client
  list            List clients

Environment variables (for setup):
  VPN_SERVER_IP   Public server IP (required)
  VPN_PORT        External port (default 443)
EOF
}

case "${1:-}" in
    setup)  cmd_setup ;;
    create) cmd_create "$2" ;;
    revoke) cmd_revoke "$2" ;;
    list)   cmd_list ;;
    help|-h|--help) print_usage ;;
    *)      print_usage; exit 1 ;;
esac

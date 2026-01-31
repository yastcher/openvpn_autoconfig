#!/usr/bin/env bash
set -euo pipefail

# ─── Load .env ────────────────────────────────────────────────
if [ ! -f .env ]; then
    echo "⚠  .env не найден. Копирую из .env.example..."
    cp .env.example .env
    echo "✏  Отредактируй .env (укажи VPN_SERVER_IP) и запусти скрипт снова."
    exit 1
fi

source .env

if [ "$VPN_SERVER_IP" = "YOUR_SERVER_IP" ] || [ -z "${VPN_SERVER_IP:-}" ]; then
    echo "❌ Укажи VPN_SERVER_IP в .env"
    exit 1
fi

PORT="${VPN_PORT:-1194}"

# ─── Check if already initialized ────────────────────────────
if [ -d "./data/pki" ]; then
    echo "⚠  PKI уже инициализирован (./data/pki существует)."
    echo "   Если хочешь начать заново: rm -rf ./data && ./setup.sh"
    exit 1
fi

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║  OpenVPN Server Setup                        ║"
echo "║  IP:   $VPN_SERVER_IP"
echo "║  Port: $PORT/udp"
echo "╚══════════════════════════════════════════════╝"
echo ""

# ─── Generate server config ──────────────────────────────────
echo "==> [1/3] Генерация конфига сервера..."
docker compose run --rm openvpn ovpn_genconfig \
    -u "udp://${VPN_SERVER_IP}:${PORT}" \
    -C AES-256-GCM \
    -a SHA256 \
    -T \
    -e "tls-version-min 1.2" \
    -e "tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384" \
    -p "dhcp-option DNS 1.1.1.1" \
    -p "dhcp-option DNS 1.0.0.1"

# Server always listens on 1194 inside container; Docker maps VPN_PORT→1194
docker compose run --rm openvpn sed -i 's/^port .*/port 1194/' /etc/openvpn/openvpn.conf

# ─── Initialize PKI (ECDSA, no passphrase) ───────────────────
echo ""
echo "==> [2/3] Инициализация PKI (ECDSA P-256, без пароля CA)..."
docker compose run --rm \
    -e EASYRSA_ALGO=ec \
    -e EASYRSA_CURVE=prime256v1 \
    -e EASYRSA_BATCH=1 \
    openvpn ovpn_initpki nopass

# ─── Persist EasyRSA vars for future client creation ─────────
echo ""
echo "==> [3/3] Сохранение настроек EasyRSA..."
docker compose run --rm openvpn bash -c 'cat > /etc/openvpn/pki/vars <<VARS
set_var EASYRSA_ALGO     ec
set_var EASYRSA_CURVE    prime256v1
VARS'

echo ""
echo "✅ Готово! Теперь:"
echo "   1. docker compose up -d          # запустить сервер"
echo "   2. ./create-client.sh client1    # создать клиента"
echo ""

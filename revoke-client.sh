#!/usr/bin/env bash
set -euo pipefail

CLIENT="${1:-}"

if [ -z "$CLIENT" ]; then
    echo "Использование: ./revoke-client.sh <имя_клиента>"
    exit 1
fi

echo "==> Отзыв сертификата: $CLIENT"
docker compose run --rm openvpn ovpn_revokeclient "$CLIENT"

# Remove local .ovpn if exists
rm -f "./clients/${CLIENT}.ovpn"

echo ""
echo "✅ Клиент $CLIENT отозван. Его .ovpn файл больше не работает."
echo ""

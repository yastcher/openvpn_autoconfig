#!/usr/bin/env bash
set -euo pipefail

CLIENT="${1:-}"

if [ -z "$CLIENT" ]; then
    echo "Использование: ./create-client.sh <имя_клиента>"
    echo ""
    echo "Примеры:"
    echo "  ./create-client.sh phone"
    echo "  ./create-client.sh laptop"
    echo "  ./create-client.sh friend-vasya"
    exit 1
fi

# Check that server is initialized
if [ ! -d "./data/pki" ]; then
    echo "❌ Сервер не инициализирован. Сначала запусти ./setup.sh"
    exit 1
fi

mkdir -p ./clients

# Check if client already exists
if [ -f "./clients/${CLIENT}.ovpn" ]; then
    echo "⚠  Файл ./clients/${CLIENT}.ovpn уже существует."
    echo "   Удали его или выбери другое имя."
    exit 1
fi

echo "==> Создание сертификата для: $CLIENT"
docker compose run --rm \
    -e EASYRSA_ALGO=ec \
    -e EASYRSA_CURVE=prime256v1 \
    openvpn easyrsa build-client-full "$CLIENT" nopass

echo "==> Экспорт .ovpn файла..."
docker compose run --rm openvpn ovpn_getclient "$CLIENT" > "./clients/${CLIENT}.ovpn"

echo ""
echo "✅ Готово: ./clients/${CLIENT}.ovpn"
echo ""
echo "Скопируй файл на устройство клиента и импортируй в OpenVPN Connect."
echo "Файл содержит все ключи — храни его как пароль!"
echo ""

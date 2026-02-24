# OpenVPN Docker Server

[English](#english) | [Русский](#русский)

---

## English

Minimal OpenVPN server in Docker with a shell-based CLI.

### Quick Start

```bash
git clone https://github.com/yastcher/openvpn_autoconfig.git ~/openvpn && cd ~/openvpn

cp .env.example .env
nano .env                        # set VPN_SERVER_IP

docker compose build             # ~15 seconds, no compilation
docker compose run --rm openvpn vpn setup   # initialize PKI
docker compose up -d             # start server
```

### Create Client

```bash
docker compose exec openvpn vpn create phone
```

File `./clients/phone.ovpn` — import into OpenVPN Connect.

### All Commands

```bash
docker compose exec openvpn vpn create <name>    # create client
docker compose exec openvpn vpn revoke <name>    # revoke client
docker compose exec openvpn vpn list             # list clients
```

If you will remove `duplicate-cn` from the client `.ovpn`
connection is bound to one device at a time — a second connection with the same
certificate will disconnect the first.

### IPv6

To assign IPv6 addresses to VPN clients, add to `.env`:

```
VPN_IPV6_SUBNET=fd00:0:0:1::/64
```

Then re-initialize the server (`setup`). The server uses NAT66 — clients get a private ULA
address and exit through the VPS's real IPv6 address, same as IPv4 works.

### Server Management

```bash
docker compose up -d         # start
docker compose down          # stop
docker compose logs -f       # logs
docker compose build         # rebuild
```

### Full Reset

```bash
docker compose down
rm -rf data/ clients/
# then: build → setup → up
```

### Structure

```
.
├── vpn.sh                # CLI tool (shell)
├── entrypoint.sh
├── Dockerfile            # Single-stage: alpine + openvpn + easy-rsa
├── docker-compose.yml
├── .env.example
├── data/                 # (auto) PKI, server config
└── clients/              # (auto) client .ovpn files
```

### Cryptography

| Parameter      | Value                                 |
|----------------|---------------------------------------|
| Certificates   | ECDSA P-256                           |
| Data channel   | AES-256-GCM                           |
| HMAC           | SHA256                                |
| TLS control    | tls-crypt                             |
| TLS min        | 1.2                                   |
| Key exchange   | ECDHE-ECDSA-AES-256-GCM-SHA384       |
| DNS (IPv4)     | 1.1.1.1 / 1.0.0.1 (Cloudflare)       |
| DNS (IPv6)     | 2606:4700:4700::1111 (if IPv6 enabled)|

---

## Русский

Минимальный OpenVPN-сервер в Docker с CLI на shell.

### Быстрый старт

```bash
git clone https://github.com/yastcher/openvpn_autoconfig.git ~/openvpn && cd ~/openvpn

cp .env.example .env
nano .env                        # вписать VPN_SERVER_IP

docker compose build             # ~15 секунд, без компиляции
docker compose run --rm openvpn vpn setup   # инициализация PKI
docker compose up -d             # запуск сервера
```

### Создание клиента

```bash
docker compose exec openvpn vpn create phone
```

Файл `./clients/phone.ovpn` — импортируй в OpenVPN Connect.

### Все команды

```bash
docker compose exec openvpn vpn create <имя>    # создать клиента
docker compose exec openvpn vpn revoke <имя>    # отозвать клиента
docker compose exec openvpn vpn list             # список клиентов
```

Если удалить `duplicate-cn` из `.ovpn`, то каждое подключение
будет привязано к одному устройству — и второе подключение
с тем же сертификатом будет разорывать первое.

### IPv6

Чтобы выдавать клиентам IPv6-адреса, добавь в `.env`:

```
VPN_IPV6_SUBNET=fd00:0:0:1::/64
```

После этого пересоздай сервер (`setup`). Используется NAT66 — клиенты получают приватный
ULA-адрес и выходят наружу через реальный IPv6 VPS, точно так же как работает IPv4.

### Управление сервером

```bash
docker compose up -d         # запустить
docker compose down          # остановить
docker compose logs -f       # логи
docker compose build         # пересобрать
```

### Полный сброс

```bash
docker compose down
rm -rf data/ clients/
# затем заново: build → setup → up
```

### Структура

```
.
├── vpn.sh                # CLI утилита (shell)
├── entrypoint.sh
├── Dockerfile            # Однослойный: alpine + openvpn + easy-rsa
├── docker-compose.yml
├── .env.example
├── data/                 # (auto) PKI, серверный конфиг
└── clients/              # (auto) .ovpn файлы клиентов
```

### Криптография

| Параметр       | Значение                              |
|----------------|---------------------------------------|
| Сертификаты    | ECDSA P-256                           |
| Data channel   | AES-256-GCM                           |
| HMAC           | SHA256                                |
| TLS control    | tls-crypt                             |
| TLS min        | 1.2                                   |
| Key exchange   | ECDHE-ECDSA-AES-256-GCM-SHA384       |
| DNS (IPv4)     | 1.1.1.1 / 1.0.0.1 (Cloudflare)       |
| DNS (IPv6)     | 2606:4700:4700::1111 (при включённом IPv6) |

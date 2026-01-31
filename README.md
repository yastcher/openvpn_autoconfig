# OpenVPN Docker Server

[English](#english) | [Русский](#русский)

---

## English

Minimal OpenVPN server in Docker. Go CLI is compiled inside the container.

### Quick Start

```bash
git clone <repo-url> ~/openvpn && cd ~/openvpn

cp .env.example .env
nano .env                        # set VPN_SERVER_IP

docker compose build             # compile Go CLI + build image
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
# The `vpn` utility is available inside the container:
docker compose exec openvpn vpn create <name>    # create client
docker compose exec openvpn vpn revoke <name>    # revoke client
docker compose exec openvpn vpn list             # list clients
```

### Server Management

```bash
docker compose up -d         # start
docker compose down          # stop
docker compose logs -f       # logs
docker compose build         # rebuild after Go changes
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
├── main.go               # CLI tool (Go)
├── go.mod
├── Dockerfile            # Multi-stage: golang → kylemanna/openvpn
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
| DNS            | 1.1.1.1 / 1.0.0.1 (Cloudflare)       |

---

## Русский

Минимальный OpenVPN-сервер в Docker. CLI на Go компилируется прямо в контейнере.

### Быстрый старт

```bash
git clone <repo-url> ~/openvpn && cd ~/openvpn

cp .env.example .env
nano .env                        # вписать VPN_SERVER_IP

docker compose build             # компилирует Go CLI + собирает образ
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
# Внутри контейнера доступна утилита `vpn`:
docker compose exec openvpn vpn create <имя>    # создать клиента
docker compose exec openvpn vpn revoke <имя>    # отозвать клиента
docker compose exec openvpn vpn list             # список клиентов
```

### Управление сервером

```bash
docker compose up -d         # запустить
docker compose down          # остановить
docker compose logs -f       # логи
docker compose build         # пересобрать после изменений в Go
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
├── main.go               # CLI утилита (Go)
├── go.mod
├── Dockerfile            # Multi-stage: golang → kylemanna/openvpn
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
| DNS            | 1.1.1.1 / 1.0.0.1 (Cloudflare)       |

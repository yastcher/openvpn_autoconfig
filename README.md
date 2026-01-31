# OpenVPN Docker Server

Минимальный OpenVPN-сервер в Docker. Три команды — и работает.

## Что внутри

- **ECDSA P-256** сертификаты (как у крутых, не RSA)
- **AES-256-GCM** шифрование
- **tls-crypt** — весь control channel зашифрован, сервер невидим для сканеров
- **TLS 1.2+** минимум, ECDHE для forward secrecy
- DNS через **Cloudflare** (1.1.1.1)

## Быстрый старт

### 1. На сервере: установи Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

### 2. Склонируй репозиторий

```bash
git clone <repo-url> ~/openvpn && cd ~/openvpn
```

### 3. Задай IP

```bash
cp .env.example .env
nano .env   # укажи VPN_SERVER_IP
```

### 4. Инициализация (один раз)

```bash
./setup.sh
```

### 5. Запуск

```bash
docker compose up -d
```

### 6. Создай клиента

```bash
./create-client.sh phone
```

Файл `./clients/phone.ovpn` — импортируй в OpenVPN Connect на любом устройстве.

---

## Управление

```bash
# Ещё клиент
./create-client.sh laptop

# Отозвать доступ
./revoke-client.sh laptop

# Логи сервера
docker compose logs -f

# Остановить
docker compose down

# Полный сброс (удалит все сертификаты!)
docker compose down && rm -rf data/ clients/
```

## Нестандартный порт

Чтобы сервер слушал на порте 50679 вместо 1194, в `.env`:

```
VPN_PORT=50679
```

Не забудь открыть порт в файрволе:

```bash
sudo ufw allow 50679/udp   # для ufw
```

## Структура

```
.
├── docker-compose.yml    # Контейнер OpenVPN
├── .env.example          # Шаблон настроек
├── setup.sh              # Инициализация PKI + конфиг сервера
├── create-client.sh      # Генерация .ovpn для клиента
├── revoke-client.sh      # Отзыв клиента
├── data/                 # (создаётся автоматически) PKI + конфиг
└── clients/              # (создаётся автоматически) .ovpn файлы
```

## Безопасность

- CA без пароля — для автоматизации. Защита — доступ к серверу (SSH-ключи, файрвол).
- `.ovpn` файл = полный доступ к VPN. Храни как пароль.
- `data/` и `clients/` в `.gitignore` — не коммить ключи в git.
- Если клиент скомпрометирован — `./revoke-client.sh имя`.

## Параметры шифрования (что генерируется)

| Параметр | Значение |
|----------|----------|
| Сертификаты | ECDSA P-256 |
| Data channel | AES-256-GCM |
| HMAC | SHA256 |
| TLS control | tls-crypt (v1) |
| TLS min | 1.2 |
| Key exchange | ECDHE-ECDSA |
| CA expire | 10 лет |
| Client expire | 10 лет |

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ovpnDir    = "/etc/openvpn"
	clientsDir = "/clients"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		runSetup()
	case "create":
		requireArg(3, "vpn create <имя>")
		runCreate(os.Args[2])
	case "revoke":
		requireArg(3, "vpn revoke <имя>")
		runRevoke(os.Args[2])
	case "list":
		runList()
	case "help", "-h", "--help":
		printUsage()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`vpn — управление OpenVPN сервером

Команды:
  setup           Инициализация сервера (один раз)
  create <имя>    Создать клиента → /clients/<имя>.ovpn
  revoke <имя>    Отозвать клиента
  list            Список клиентов

Переменные окружения (для setup):
  VPN_SERVER_IP   Публичный IP сервера (обязательно)
  VPN_PORT        Внешний порт (по умолчанию 1194)
`)
}

// ──────────────────────────────────────────────
//  setup
// ──────────────────────────────────────────────

func runSetup() {
	ip := os.Getenv("VPN_SERVER_IP")
	if ip == "" || ip == "YOUR_SERVER_IP" {
		fatal("VPN_SERVER_IP не задан. Проверь .env")
	}

	port := envOr("VPN_PORT", "1194")

	if _, err := os.Stat(filepath.Join(ovpnDir, "pki")); err == nil {
		fatal("PKI уже инициализирован. Для сброса: удали ./data/ и запусти заново.")
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Printf("║  OpenVPN Setup                       ║\n")
	fmt.Printf("║  IP:   %-30s║\n", ip)
	fmt.Printf("║  Port: %-30s║\n", port+"/udp")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. Generate server config
	step("1/3", "Генерация конфига сервера")
	run("ovpn_genconfig",
		"-u", fmt.Sprintf("udp://%s:%s", ip, port),
		"-C", "AES-256-GCM",
		"-a", "SHA256",
		"-T",
		"-e", "tls-version-min 1.2",
		"-e", "tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384",
		"-p", "dhcp-option DNS 1.1.1.1",
		"-p", "dhcp-option DNS 1.0.0.1",
	)

	// Server listens on 1194 inside container; Docker maps VPN_PORT → 1194
	confPath := filepath.Join(ovpnDir, "openvpn.conf")
	run("sed", "-i", "s/^port .*/port 1194/", confPath)

	// 2. Initialize PKI with ECDSA
	step("2/3", "Инициализация PKI (ECDSA P-256, без пароля CA)")
	setEasyRSAEnv()
	os.Setenv("EASYRSA_BATCH", "1")
	run("ovpn_initpki", "nopass")

	// 3. Persist EasyRSA vars for future client creation
	step("3/3", "Сохранение настроек EasyRSA")
	varsPath := filepath.Join(ovpnDir, "pki", "vars")
	vars := "set_var EASYRSA_ALGO     ec\nset_var EASYRSA_CURVE    prime256v1\n"
	if err := os.WriteFile(varsPath, []byte(vars), 0644); err != nil {
		fatal("Не удалось записать vars: " + err.Error())
	}

	fmt.Println()
	fmt.Println("✅ Сервер инициализирован!")
	fmt.Println()
	fmt.Println("  docker compose up -d              # запустить")
	fmt.Println("  docker compose exec openvpn vpn create phone  # создать клиента")
	fmt.Println()
}

// ──────────────────────────────────────────────
//  create
// ──────────────────────────────────────────────

func runCreate(name string) {
	checkPKI()

	outPath := filepath.Join(clientsDir, name+".ovpn")
	if _, err := os.Stat(outPath); err == nil {
		fatal(fmt.Sprintf("Файл %s уже существует. Удали его или выбери другое имя.", outPath))
	}

	if err := os.MkdirAll(clientsDir, 0755); err != nil {
		fatal("Не удалось создать " + clientsDir + ": " + err.Error())
	}

	fmt.Printf("==> Создание клиента: %s\n", name)
	setEasyRSAEnv()
	run("easyrsa", "build-client-full", name, "nopass")

	fmt.Println("==> Экспорт .ovpn...")
	cmd := exec.Command("ovpn_getclient", name)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fatal("Ошибка экспорта: " + err.Error())
	}

	if err := os.WriteFile(outPath, out, 0600); err != nil {
		fatal("Не удалось записать файл: " + err.Error())
	}

	fmt.Println()
	fmt.Printf("✅ %s\n", outPath)
	fmt.Println("   Скопируй на устройство → импортируй в OpenVPN Connect.")
	fmt.Println("   Файл содержит все ключи — храни как пароль!")
	fmt.Println()
}

// ──────────────────────────────────────────────
//  revoke
// ──────────────────────────────────────────────

func runRevoke(name string) {
	checkPKI()

	certPath := filepath.Join(ovpnDir, "pki", "issued", name+".crt")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		fatal(fmt.Sprintf("Клиент '%s' не найден.", name))
	}

	fmt.Printf("==> Отзыв клиента: %s\n", name)
	os.Setenv("EASYRSA_BATCH", "1")
	run("ovpn_revokeclient", name)

	os.Remove(filepath.Join(clientsDir, name+".ovpn"))

	fmt.Println()
	fmt.Printf("✅ Клиент %s отозван. Его .ovpn больше не работает.\n", name)
	fmt.Println()
}

// ──────────────────────────────────────────────
//  list
// ──────────────────────────────────────────────

func runList() {
	checkPKI()

	issuedDir := filepath.Join(ovpnDir, "pki", "issued")
	entries, err := os.ReadDir(issuedDir)
	if err != nil {
		fatal("Не удалось прочитать " + issuedDir)
	}

	fmt.Println("Клиенты:")
	count := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".crt")
		// Skip server certificate
		if strings.HasPrefix(name, "server") {
			continue
		}

		status := "✗ ovpn не экспортирован"
		ovpnPath := filepath.Join(clientsDir, name+".ovpn")
		if _, err := os.Stat(ovpnPath); err == nil {
			status = "✓ " + ovpnPath
		}

		// Check if revoked
		revokedPath := filepath.Join(ovpnDir, "pki", "revoked", "certs_by_serial")
		if isRevoked(revokedPath, name) {
			status = "⊘ отозван"
		}

		fmt.Printf("  • %-20s %s\n", name, status)
		count++
	}

	if count == 0 {
		fmt.Println("  (пусто — создай клиента: vpn create <имя>)")
	}
	fmt.Println()
}

// ──────────────────────────────────────────────
//  helpers
// ──────────────────────────────────────────────

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fatal(fmt.Sprintf("Команда '%s' завершилась с ошибкой: %v", name, err))
	}
}

func setEasyRSAEnv() {
	os.Setenv("EASYRSA_ALGO", "ec")
	os.Setenv("EASYRSA_CURVE", "prime256v1")
}

func checkPKI() {
	if _, err := os.Stat(filepath.Join(ovpnDir, "pki")); os.IsNotExist(err) {
		fatal("Сервер не инициализирован. Сначала: docker compose run --rm openvpn vpn setup")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireArg(n int, usage string) {
	if len(os.Args) < n {
		fatal("Использование: " + usage)
	}
}

func isRevoked(revokedDir, name string) bool {
	// Simple heuristic: check CRL index
	indexPath := filepath.Join(ovpnDir, "pki", "index.txt")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "/CN="+name) && strings.HasPrefix(line, "R") {
			return true
		}
	}
	return false
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "❌ %s\n", msg)
	os.Exit(1)
}

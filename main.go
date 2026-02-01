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
	pkiDir     = "/etc/openvpn/pki"
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
		requireArg(3, "vpn create <name>")
		runCreate(os.Args[2])
	case "revoke":
		requireArg(3, "vpn revoke <name>")
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
	fmt.Print(`vpn — OpenVPN server management

Commands:
  setup           Initialize server (one time)
  create <name>   Create client → /clients/<name>.ovpn
  revoke <name>   Revoke client
  list            List clients

Environment variables (for setup):
  VPN_SERVER_IP   Public server IP (required)
  VPN_PORT        External port (default 1194)
`)
}

// ──────────────────────────────────────────────
//  setup
// ──────────────────────────────────────────────

func runSetup() {
	ip := os.Getenv("VPN_SERVER_IP")
	if ip == "" || ip == "YOUR_SERVER_IP" {
		fatal("VPN_SERVER_IP not set. Check .env")
	}

	port := envOr("VPN_PORT", "1194")

	if _, err := os.Stat(pkiDir); err == nil {
		fatal("PKI already initialized. To reset: delete ./data/ and run again.")
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Printf("║  OpenVPN Setup                       ║\n")
	fmt.Printf("║  IP:   %-30s║\n", ip)
	fmt.Printf("║  Port: %-30s║\n", port+"/udp")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	setEasyRSAEnv()

	// 1. Initialize PKI
	step("1/5", "Initializing PKI")
	run("easyrsa", "init-pki")

	// 2. Build CA (ECDSA P-256, no password)
	step("2/5", "Building CA (ECDSA P-256)")
	os.Setenv("EASYRSA_REQ_CN", "OpenVPN-CA")
	run("easyrsa", "build-ca", "nopass")

	// 3. Generate server certificate
	step("3/5", "Generating server certificate")
	os.Unsetenv("EASYRSA_REQ_CN")
	run("easyrsa", "build-server-full", "server", "nopass")

	// 4. Generate tls-crypt key + initial CRL
	step("4/5", "Generating tls-crypt key and CRL")
	taKeyPath := filepath.Join(pkiDir, "ta.key")
	run("openvpn", "--genkey", "--secret", taKeyPath)
	run("easyrsa", "gen-crl")
	os.Chmod(filepath.Join(pkiDir, "crl.pem"), 0644)

	// 5. Write server config
	step("5/5", "Writing server config")

	serverConf := fmt.Sprintf(`port 1194
proto udp
dev tun
ca %[1]s/ca.crt
cert %[1]s/issued/server.crt
key %[1]s/private/server.key
dh none
topology subnet
server 192.168.255.0 255.255.255.0
ifconfig-pool-persist /etc/openvpn/ipp.txt
push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 1.0.0.1"
push "block-outside-dns"
keepalive 10 120
cipher AES-256-GCM
auth SHA256
tls-crypt %[1]s/ta.key
tls-version-min 1.2
tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384
crl-verify %[1]s/crl.pem
persist-key
persist-tun
status /etc/openvpn/status.log
verb 3
explicit-exit-notify 1
`, pkiDir)

	confPath := filepath.Join(ovpnDir, "openvpn.conf")
	if err := os.WriteFile(confPath, []byte(serverConf), 0644); err != nil {
		fatal("Failed to write server config: " + err.Error())
	}

	// Save connection info for client generation
	vpnEnv := fmt.Sprintf("VPN_SERVER_IP=%s\nVPN_PORT=%s\n", ip, port)
	os.WriteFile(filepath.Join(ovpnDir, "vpn.env"), []byte(vpnEnv), 0644)

	// Save EasyRSA vars for future use
	varsPath := filepath.Join(pkiDir, "vars")
	vars := "set_var EASYRSA_ALGO     ec\nset_var EASYRSA_CURVE    prime256v1\n"
	os.WriteFile(varsPath, []byte(vars), 0644)

	fmt.Println()
	fmt.Println("✅ Server initialized!")
	fmt.Println()
	fmt.Println("  docker compose up -d                              # start server")
	fmt.Println("  docker compose exec openvpn vpn create phone      # create client")
	fmt.Println()
}

// ──────────────────────────────────────────────
//  create
// ──────────────────────────────────────────────

func runCreate(name string) {
	checkPKI()

	outPath := filepath.Join(clientsDir, name+".ovpn")
	if _, err := os.Stat(outPath); err == nil {
		fatal(fmt.Sprintf("File %s already exists. Delete it or choose another name.", outPath))
	}

	if err := os.MkdirAll(clientsDir, 0755); err != nil {
		fatal("Failed to create " + clientsDir + ": " + err.Error())
	}

	ip, port := loadVPNEnv()

	fmt.Printf("==> Creating client: %s\n", name)
	setEasyRSAEnv()
	run("easyrsa", "build-client-full", name, "nopass")

	fmt.Println("==> Assembling .ovpn...")

	caCert := readFile(filepath.Join(pkiDir, "ca.crt"))
	clientCert := extractPEM(readFile(filepath.Join(pkiDir, "issued", name+".crt")), "CERTIFICATE")
	clientKey := readFile(filepath.Join(pkiDir, "private", name+".key"))
	taKey := readFile(filepath.Join(pkiDir, "ta.key"))

	ovpn := fmt.Sprintf(`client
dev tun
proto udp
remote %s %s
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
explicit-exit-notify
<ca>
%s</ca>
<cert>
%s</cert>
<key>
%s</key>
<tls-crypt>
%s</tls-crypt>
`, ip, port, caCert, clientCert, clientKey, taKey)

	if err := os.WriteFile(outPath, []byte(ovpn), 0600); err != nil {
		fatal("Failed to write file: " + err.Error())
	}

	fmt.Println()
	fmt.Printf("✅ %s\n", outPath)
	fmt.Println("   Copy to device → import into OpenVPN Connect.")
	fmt.Println("   File contains all keys — store like a password!")
	fmt.Println()
}

// ──────────────────────────────────────────────
//  revoke
// ──────────────────────────────────────────────

func runRevoke(name string) {
	checkPKI()

	certPath := filepath.Join(pkiDir, "issued", name+".crt")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		fatal(fmt.Sprintf("Client '%s' not found.", name))
	}

	fmt.Printf("==> Revoking client: %s\n", name)
	setEasyRSAEnv()
	run("easyrsa", "revoke", name)
	run("easyrsa", "gen-crl")
	os.Chmod(filepath.Join(pkiDir, "crl.pem"), 0644)

	os.Remove(filepath.Join(clientsDir, name+".ovpn"))

	fmt.Println()
	fmt.Printf("✅ Client %s revoked. Its .ovpn no longer works.\n", name)
	fmt.Println()
}

// ──────────────────────────────────────────────
//  list
// ──────────────────────────────────────────────

func runList() {
	checkPKI()

	issuedDir := filepath.Join(pkiDir, "issued")
	entries, err := os.ReadDir(issuedDir)
	if err != nil {
		fatal("Failed to read " + issuedDir)
	}

	fmt.Println("Clients:")
	count := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".crt")
		if name == "server" {
			continue
		}

		status := "✗ ovpn not exported"
		ovpnPath := filepath.Join(clientsDir, name+".ovpn")
		if _, err := os.Stat(ovpnPath); err == nil {
			status = "✓ " + ovpnPath
		}

		if isRevoked(name) {
			status = "⊘ revoked"
		}

		fmt.Printf("  • %-20s %s\n", name, status)
		count++
	}

	if count == 0 {
		fmt.Println("  (empty — create client: vpn create <name>)")
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
		fatal(fmt.Sprintf("Command '%s %s' failed: %v", name, strings.Join(args, " "), err))
	}
}

func setEasyRSAEnv() {
	os.Setenv("EASYRSA_PKI", pkiDir)
	os.Setenv("EASYRSA_BATCH", "1")
	os.Setenv("EASYRSA_ALGO", "ec")
	os.Setenv("EASYRSA_CURVE", "prime256v1")
}

func checkPKI() {
	if _, err := os.Stat(pkiDir); os.IsNotExist(err) {
		fatal("Server not initialized. First run: docker compose run --rm openvpn vpn setup")
	}
}

func loadVPNEnv() (ip, port string) {
	ip = os.Getenv("VPN_SERVER_IP")
	port = os.Getenv("VPN_PORT")

	// Fallback: read from saved file (useful for docker compose exec)
	if ip == "" || port == "" {
		data, _ := os.ReadFile(filepath.Join(ovpnDir, "vpn.env"))
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VPN_SERVER_IP=") && ip == "" {
				ip = strings.TrimPrefix(line, "VPN_SERVER_IP=")
			}
			if strings.HasPrefix(line, "VPN_PORT=") && port == "" {
				port = strings.TrimPrefix(line, "VPN_PORT=")
			}
		}
	}

	if ip == "" {
		fatal("VPN_SERVER_IP not set")
	}
	if port == "" {
		port = "1194"
	}
	return
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("Failed to read " + path + ": " + err.Error())
	}
	return string(data)
}

// extractPEM returns only the PEM block of the given type.
// EasyRSA cert files contain human-readable text before the PEM block.
func extractPEM(data, pemType string) string {
	begin := "-----BEGIN " + pemType + "-----"
	end := "-----END " + pemType + "-----"
	startIdx := strings.Index(data, begin)
	endIdx := strings.Index(data, end)
	if startIdx == -1 || endIdx == -1 {
		return data
	}
	return data[startIdx : endIdx+len(end)] + "\n"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireArg(n int, usage string) {
	if len(os.Args) < n {
		fatal("Usage: " + usage)
	}
}

func isRevoked(name string) bool {
	indexPath := filepath.Join(pkiDir, "index.txt")
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

func step(num, msg string) {
	fmt.Printf("==> [%s] %s\n", num, msg)
}

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

	if _, err := os.Stat(filepath.Join(ovpnDir, "pki")); err == nil {
		fatal("PKI already initialized. To reset: delete ./data/ and run again.")
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Printf("║  OpenVPN Setup                       ║\n")
	fmt.Printf("║  IP:   %-30s║\n", ip)
	fmt.Printf("║  Port: %-30s║\n", port+"/udp")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. Generate server config using proper flags so ovpn_getclient
	//    also produces correct client configs (tls-crypt, cipher, auth, DNS).
	//    NOTE: ovpn_genconfig has parsing bugs with -e, so we only pass
	//    well-supported flags here and append extra directives to the file.
	step("1/3", "Generating server config")
	run("ovpn_genconfig",
		"-u", fmt.Sprintf("udp://%s:%s", ip, port),
		"-C", "AES-256-GCM",
		"-a", "SHA256",
		"-T",
		"-n", "1.1.1.1",
		"-n", "1.0.0.1",
	)

	// Patch config: internal port always 1194, ECDSA needs dh none
	confPath := filepath.Join(ovpnDir, "openvpn.conf")
	run("sed", "-i", "s/^port .*/port 1194/", confPath)
	run("sed", "-i", "s/^dh dh.pem/dh none/", confPath)

	// Append TLS hardening (can't use -e flags — ovpn_genconfig mangles them)
	extraConfig := "\ntls-version-min 1.2\ntls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384\n"
	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fatal("Failed to open config: " + err.Error())
	}
	if _, err := f.WriteString(extraConfig); err != nil {
		f.Close()
		fatal("Failed to append config: " + err.Error())
	}
	f.Close()

	validateConfig(confPath)

	// 2. Initialize PKI with ECDSA
	step("2/3", "Initializing PKI (ECDSA P-256, no CA password)")
	setEasyRSAEnv()
	os.Setenv("EASYRSA_BATCH", "1")
	run("ovpn_initpki", "nopass")

	// 3. Persist EasyRSA vars for future client creation
	step("3/3", "Saving EasyRSA settings")
	varsPath := filepath.Join(ovpnDir, "pki", "vars")
	vars := "set_var EASYRSA_ALGO     ec\nset_var EASYRSA_CURVE    prime256v1\n"
	if err := os.WriteFile(varsPath, []byte(vars), 0644); err != nil {
		fatal("Failed to write vars: " + err.Error())
	}

	fmt.Println()
	fmt.Println("✅ Server initialized!")
	fmt.Println()
	fmt.Println("  docker compose up -d              # start server")
	fmt.Println("  docker compose exec openvpn vpn create phone  # create client")
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

	fmt.Printf("==> Creating client: %s\n", name)
	setEasyRSAEnv()
	run("easyrsa", "build-client-full", name, "nopass")

	fmt.Println("==> Exporting .ovpn...")
	cmd := exec.Command("ovpn_getclient", name)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fatal("Export error: " + err.Error())
	}

	if err := os.WriteFile(outPath, out, 0600); err != nil {
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

	certPath := filepath.Join(ovpnDir, "pki", "issued", name+".crt")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		fatal(fmt.Sprintf("Client '%s' not found.", name))
	}

	fmt.Printf("==> Revoking client: %s\n", name)
	os.Setenv("EASYRSA_BATCH", "1")
	run("ovpn_revokeclient", name)

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

	issuedDir := filepath.Join(ovpnDir, "pki", "issued")
	entries, err := os.ReadDir(issuedDir)
	if err != nil {
		fatal("Failed to read " + issuedDir)
	}

	fmt.Println("Clients:")
	count := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".crt")
		// Skip server certificate
		if strings.HasPrefix(name, "server") {
			continue
		}

		status := "✗ ovpn not exported"
		ovpnPath := filepath.Join(clientsDir, name+".ovpn")
		if _, err := os.Stat(ovpnPath); err == nil {
			status = "✓ " + ovpnPath
		}

		// Check if revoked
		revokedPath := filepath.Join(ovpnDir, "pki", "revoked", "certs_by_serial")
		if isRevoked(revokedPath, name) {
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
		fatal(fmt.Sprintf("Command '%s' failed: %v", name, err))
	}
}

func setEasyRSAEnv() {
	os.Setenv("EASYRSA_ALGO", "ec")
	os.Setenv("EASYRSA_CURVE", "prime256v1")
}

func checkPKI() {
	if _, err := os.Stat(filepath.Join(ovpnDir, "pki")); os.IsNotExist(err) {
		fatal("Server not initialized. First run: docker compose run --rm openvpn vpn setup")
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
		fatal("Usage: " + usage)
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

func step(num, msg string) {
	fmt.Printf("==> [%s] %s\n", num, msg)
}

func validateConfig(confPath string) {
	data, err := os.ReadFile(confPath)
	if err != nil {
		fatal("Cannot read config for validation: " + err.Error())
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "tls-cipher") {
			found = true
			if !strings.Contains(line, "TLS-ECDHE-") {
				fatal("Bad tls-cipher in config: " + line)
			}
		}
	}
	if !found {
		fatal("tls-cipher directive missing from config")
	}
}

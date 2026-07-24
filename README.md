# net---cat
# Secure TCP Chat Server (Advanced NetCat Group Chat)

An enterprise-ready, concurrent, multi-room group chat architecture written in Go that recreates and enhances the classic system utility `nc` (NetCat). Built on a modular sub-package architecture using pure standard library components, this system supports **TLS/SSL Encryption**, **Dynamic JSON Configurations**, **Password-Locked Chatrooms**, **Inactivity Supervision**, **Private Messaging**, and **Structured SQL Database Logging**.

## 🚀 Key Features

*   **Secure TLS/SSL Transport Layer:** All data pipes use strict encryption. Normal insecure network scanners are rejected automatically.
*   **Decoupled Modular Package Design:** Separated into `config`, `db`, `models`, and `server` modules for production maintainability.
*   **Isolate Chatrooms with Optional Passwords:** Move smoothly between public environments or lock private rooms using `/join room_name password`.
*   **Zero Local Echo Artefacts:** The broadcast engine filters the sender's channel footprint so outbound messages appear exactly once.
*   **Two-Stage Inactivity Monitor:** A dedicated loop audits idle connection vectors, alerts passive typers, and runs automated kicks.
*   **Structured Persistence Logging:** All system state updates, private messages, and broadcasts are logged as transaction queries in an append-only datastore (`chat_archive.db`).
*   **Robust Content Filtration Middleware:** Real-time profanity screening replaces flagged tokens across public and private pipes.

---

## 🛠️ Command Toolkit Guide

Inside the interactive chat terminal, any text string prefixed with a forward slash (`/`) is intercepted and routed through the command processor:

| Command | Arguments | Permissions | Description |
| :--- | :--- | :--- | :--- |
| `/rooms` | None | Public | Lists all active rooms, active player counts, and visibility status. |
| `/join` | `<room_name> [password]` | Public | Swaps rooms. Auto-creates new rooms. Adds a password lock if an extra string is provided. |
| `/leave` | None | Public | Instantly routes your connection profile back into the master default `lobby`. |
| `/msg` | `<username> <message>` | Public | Sends an encrypted peer-to-peer private direct message across room boundaries. |
| `/admin` | `<secret_password>` | Public | Validates credentials against your JSON schema to elevate your connection. |
| `/kick` | `<username>` | Admin Only | Forcefully disconnects a target user profile from the chat network. |
| `/ban` | `<username>` | Admin Only | Drops a target client and maps their remote host IP into a permanent ban pool. |

---

## 📦 Directory Structure

```text
TCPChat/
├── go.mod                 # Go module definition file
├── config.json            # Dynamic JSON configuration parameters
├── main.go                # Central orchestrator and listener entrypoint
├── main_test.go           # Integration vectors verifying system pipelines
├── config/
│   └── config.go          # Config reflection schemas and static logo text
├── db/
│   └── database.go        # SQL transaction logger interface 
├── models/
│   └── client.go          # Thread-isolated client state models
└── server/
    ├── server.go          # Main broadcasting engine and background loops
    └── commands.go        # Command parsers and profanity filters
```

---

## 🏎️ Getting Started

### 1. Prerequisites
Ensure you have [Go](https://go.dev) (version 1.18 or higher) and [OpenSSL](https://openssl.org) installed on your machine.

### 2. Generate Local TLS Cryptographic Certificates
Because the application runs on an encrypted TLS transport layer, generate your local public/private development key pair inside your project root:
```bash
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes -subj "/CN=localhost"
```

### 3. Verify Configuration Matrix
Ensure a file named `config.json` is sitting in your root execution workspace. It should look like this:
```json
{
  "default_port": "8989",
  "max_clients": 10,
  "time_format": "2006-01-02 15:04:05",
  "admin_secret": "SuperSecureAdminPass123",
  "idle_timeout_seconds": 300,
  "warning_threshold_seconds": 240,
  "cert_file": "server.crt",
  "key_file": "server.key",
  "banned_words": ["badword1", "badword2", "spamtext"]
}
```

### 4. Boot the Server
Compile and launch the main application package:
```bash
# Option A: Boot using the default port declared inside your config JSON
go run main.go

# Option B: Override port parameters dynamically via a command-line flag
go run main.go 2525
```

### 5. Establish Client Handshakes
Standard `nc` (Netcat) does not handle TLS transport layers out-of-the-box. Connect safely from your client terminal windows using the OpenSSL client utility:
```bash
openssl s_client -connect localhost:8989 -quiet
```

---

## 🧪 Testing and Verification

The package ecosystem features independent mock objects protecting network pipelines. Run integration test scripts across all subdirectory paths by executing:
```bash
go test -v ./...
```

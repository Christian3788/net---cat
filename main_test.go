package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"TCPChat/config"
	"TCPChat/db"
	"TCPChat/server"
)

func TestSecureServerSuite(t *testing.T) {
	// 1. Setup temporary configurations to avoid breaking global config.json parameters
	cfg := &config.AppConfig{
		DefaultPort:             "0", // Grabs a clean ephemeral port from the OS
		MaxClients:              5,
		TimeFormat:              "2006-01-02 15:04:05",
		AdminSecret:             "TestAdminSecret123",
		IdleTimeoutSeconds:      4,
		WarningThresholdSeconds: 2,
		CertFile:                "server.crt",
		KeyFile:                 "server.key",
		BannedWords:             []string{"badword1"},
		IdleTimeout:             4 * time.Second,
		WarningThreshold:        2 * time.Second,
	}

	// 2. Initialize test database footprint
	database, err := db.InitDB()
	if err != nil {
		t.Fatalf("Failed to initialize test logs database: %v", err)
	}
	defer database.Close()
	defer os.Remove("chat_archive.db") // Cleanup file database after test execution completes

	// 3. Bind TLS socket pipeline listener
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		t.Fatalf("TLS Test initialization failed. Please make sure you generated server.crt and server.key: %v", err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to bind encrypted test listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	srv := server.NewServer(cfg, database)

	// Spin up server background engines
	go srv.Run()
	go srv.MonitorInactivity()

	// Spin up connection listener handler loop
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go srv.HandleConnection(conn)
		}
	}()

	// --- TEST CLIENT 1 (Alice) ---
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	tlsDialer := &tls.Config{InsecureSkipVerify: true} // Skip verification for self-signed certificates

	conn1, err := tls.DialWithDialer(dialer, "tcp", addr, tlsDialer)
	if err != nil {
		t.Fatalf("Secure Client 1 failed to handshake: %v", err)
	}
	defer conn1.Close()

	r1 := bufio.NewReader(conn1)
	_, _ = r1.ReadString(':') // Consume initialization logo text string
	fmt.Fprintln(conn1, "Alice")
	time.Sleep(50 * time.Millisecond)

	// --- TEST CLIENT 2 (Bob) ---
	conn2, err := tls.DialWithDialer(dialer, "tcp", addr, tlsDialer)
	if err != nil {
		t.Fatalf("Secure Client 2 failed to handshake: %v", err)
	}
	defer conn2.Close()

	r2 := bufio.NewReader(conn2)
	_, _ = r2.ReadString(':')
	fmt.Fprintln(conn2, "Bob")
	time.Sleep(50 * time.Millisecond)

	// 4. TEST CASE A: Verify Cross-Room Message Isolation and Profanity Filters
	fmt.Fprintln(conn1, "/join dynamic_lounge")
	time.Sleep(50 * time.Millisecond)

	// Clear out Bob's entry messages
	go func() {
		buf := make([]byte, 1024)
		_, _ = r2.Read(buf)
	}()

	// Alice transmits message inside isolated channel containing bad words
	fmt.Fprintln(conn1, "This is a badword1 text payload.")
	time.Sleep(50 * time.Millisecond)

	// Bob is still sitting inside 'lobby' and should NOT capture Alice's transmission
	conn2.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	bufBob := make([]byte, 512)
	n, _ := r2.Read(bufBob)
	bobOutput := string(bufBob[:n])

	if strings.Contains(bobOutput, "dynamic_lounge") || strings.Contains(bobOutput, "badword1") {
		t.Error("Security leak: Room boundary isolation model or profanity filter failed.")
	}

	// 5. TEST CASE B: Verify Cross-Boundary Direct Messages (/msg)
	fmt.Fprintln(conn1, "/msg Bob Hey there Robert!")
	time.Sleep(50 * time.Millisecond)

	conn2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, _ = r2.Read(bufBob)
	bobOutput = string(bufBob[:n])

	if !strings.Contains(bobOutput, "PM from Alice") || !strings.Contains(bobOutput, "Hey there Robert!") {
		t.Error("Routing failure: /msg command engine failed to deliver direct message across room boundaries.")
	}

	// 6. TEST CASE C: Verify Inactivity Supervision loop drops idle users
	// We let the clock run out for Alice without any input signals
	time.Sleep(5 * time.Second)

	bufAlice := make([]byte, 512)
	conn1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = r1.Read(bufAlice)

	aliceOutput := string(bufAlice)
	if !strings.Contains(aliceOutput, "inactivity") {
		t.Error("Failure: Automated background supervisor worker failed to kick inactive connection.")
	}
}

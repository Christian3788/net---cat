package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"

	"TCPChat/config"
	"TCPChat/db"
	"TCPChat/server"
)

func main() {
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Critical Error loading configuration file: %v", err)
	}

	port := cfg.DefaultPort
	args := os.Args[1:]
	if len(args) == 1 {
		port = args[0]
	} else if len(args) > 1 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		return
	}

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Critical Error initializing data engine: %v", err)
	}
	defer database.Close()

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		log.Fatalf("TLS Configuration Exception: Key pair parsing failed: %v. Please make sure keys are generated.", err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", ":"+port, tlsConfig)
	if err != nil {
		log.Fatalf("Error starting TLS server socket: %v", err)
	}
	defer listener.Close()

	fmt.Printf("Secure TLS Chat Listening on the port :%s\n", port)

	srv := server.NewServer(cfg, database)
	go srv.Run()
	go srv.MonitorInactivity()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		srv.Mutex.Lock()
		remoteIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if srv.BannedIPs[remoteIP] {
			fmt.Fprintln(conn, "You are permanently banned from this server.")
			conn.Close()
			srv.Mutex.Unlock()
			continue
		}
		srv.Mutex.Unlock()

		go srv.HandleConnection(conn)
	}
}

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

// runSockholdMain is entered when the binary is re-exec'd with --sockhold.
// It opens the requested socket inside whatever network namespace the process
// was started in, signals "ready\n" to the parent, then blocks indefinitely.
func runSockholdMain() {
	fs := flag.NewFlagSet("sockhold", flag.ExitOnError)
	sockType := fs.String("type", "", "tcp-listen | udp-bind | tcp-connect")
	port := fs.Int("port", 0, "port number")
	addr := fs.String("addr", "", "remote address for tcp-connect")
	fs.Parse(os.Args[2:]) // os.Args[1] == "--sockhold"

	switch *sockType {
	case "tcp-listen":
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
		if err != nil {
			log.Fatalf("sockhold: tcp listen :%d: %v", *port, err)
		}
		defer ln.Close()
		signalReady()
		// Accept and immediately close connections; we only need the socket
		// to appear in ss output.
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}

	case "udp-bind":
		conn, err := net.ListenPacket("udp", fmt.Sprintf("0.0.0.0:%d", *port))
		if err != nil {
			log.Fatalf("sockhold: udp bind :%d: %v", *port, err)
		}
		defer conn.Close()
		signalReady()
		select {} // block forever

	case "tcp-connect":
		conn, err := net.Dial("tcp", net.JoinHostPort(*addr, fmt.Sprintf("%d", *port)))
		if err != nil {
			log.Fatalf("sockhold: tcp connect %s:%d: %v", *addr, *port, err)
		}
		defer conn.Close()
		signalReady()
		select {} // block forever

	default:
		log.Fatalf("sockhold: unknown type %q (want tcp-listen, udp-bind, tcp-connect)", *sockType)
	}
}

// signalReady writes "ready\n" to stdout then closes stdout so the parent
// daemon gets EOF on its pipe and knows the socket is bound.
func signalReady() {
	fmt.Fprintln(os.Stdout, "ready")
	os.Stdout.Close()
}

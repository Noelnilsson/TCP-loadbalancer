package backend

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

// StartServer starts a simple echo server on the given address.
// Blocks until the server is stopped, so run in a goroutine.
func StartServer(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start backend server: %w", err)
	}
	defer listener.Close()

	log.Printf("[Backend %s] Listening", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[Backend %s] Accept error: %v", address, err)
			continue
		}

		go handleConnection(conn, address)
	}
}

// handleConnection handles a single client connection.
// It reads lines from the client and echoes them back with the server ID.
func handleConnection(conn net.Conn, address string) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	log.Printf("[Backend %s] New connection from %s", address, clientAddr)

	welcome := fmt.Sprintf("Connected to Backend %s\n", address)
	conn.Write([]byte(welcome))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[Backend %s] Received: %s", address, line)

		response := fmt.Sprintf("[Backend %s] Echo: %s\n", address, line)
		conn.Write([]byte(response))
	}

	log.Printf("[Backend %s] Connection closed from %s", address, clientAddr)
}

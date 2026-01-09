package backend

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

// StartServer starts an echo server on the backend address.
func StartServer(b *Backend) error {
	listener, err := net.Listen("tcp", b.getAddress())
	if err != nil {
		return fmt.Errorf("failed to start backend server: %w", err)
	}
	defer listener.Close()

	log.Printf("[Backend %s] Listening", b.getAddress())

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[Backend %s] Accept error: %v", b.getAddress(), err)
			continue
		}

		// Check if the server is "alive". If not, wait until it is resurrected.
		b.mu.Lock()
		for !b.Alive {
			b.cond.Wait()
		}
		b.mu.Unlock()

		go handleConnection(conn, b.getAddress())
	}
}

// handleConnection echoes lines back to the client.
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

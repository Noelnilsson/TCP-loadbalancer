package proxy

import (
	"io"
	"net"
	"sync"
	"time"
)

// Proxy handles bidirectional data transfer between a client and a backend.
// It copies data in both directions simultaneously until one side closes.
func Proxy(client net.Conn, backend net.Conn) error {
	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)

	// Client -> Backend
	go func() {
		defer wg.Done()
		_, err := io.Copy(backend, client)
		if err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	// Backend -> Client
	go func() {
		defer wg.Done()
		_, err := io.Copy(client, backend)
		if err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	return nil
}

type countingWriter struct {
	w     io.Writer
	count int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.count += int64(n)
	return n, err
}

// ProxyWithStats is like Proxy but also tracks bytes transferred.
// Returns the number of bytes sent (client->backend) and received (backend->client).
func ProxyWithStats(client net.Conn, backend net.Conn) (bytesSent int64, bytesReceived int64, err error) {
	toBackend := &countingWriter{w: backend}
	toClient := &countingWriter{w: client}

	var wg sync.WaitGroup
	wg.Add(2)

	errCh := make(chan error, 2)

	go func() {
		defer wg.Done()
		_, copyErr := io.Copy(toBackend, client)
		if copyErr != nil {
			errCh <- copyErr
		}
	}()

	go func() {
		defer wg.Done()
		_, copyErr := io.Copy(toClient, backend)
		if copyErr != nil {
			errCh <- copyErr
		}
	}()

	wg.Wait()
	close(errCh)

	var finalErr error
	for e := range errCh {
		if finalErr == nil {
			finalErr = e
		}
	}

	return toBackend.count, toClient.count, finalErr
}

// copyData copies data from src to dst until EOF or error.
// Signals EOF to the other end by calling CloseWrite on TCP connections.
func copyData(dst net.Conn, src net.Conn, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	_, err := io.Copy(dst, src)
	if err != nil && err != io.EOF {
		errCh <- err
	}

	// Signal EOF to the other end
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}
}

// copyDataWithBuffer is like copyData but uses a custom buffer size.
// Useful for tuning performance based on expected traffic patterns.
func copyDataWithBuffer(dst net.Conn, src net.Conn, bufferSize int) (int64, error) {
	buf := make([]byte, bufferSize)
	return io.CopyBuffer(dst, src, buf)
}

// SetDeadlines sets read/write deadlines on both connections.
// This prevents connections from hanging forever if one side stops responding.
func SetDeadlines(client net.Conn, backend net.Conn, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	if err := client.SetDeadline(deadline); err != nil {
		return err
	}

	if err := backend.SetDeadline(deadline); err != nil {
		return err
	}

	return nil
}

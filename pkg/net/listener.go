// Package net provides networking utilities for the Minecraft server.
package net

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// ConnectionHandler handles new connections.
type ConnectionHandler interface {
	HandleConnection(conn *Connection)
}

// Listener wraps a TCP listener for Minecraft connections.
type Listener struct {
	addr     string
	listener net.Listener
	handler  ConnectionHandler
	wg       sync.WaitGroup
	closed   bool
	mu       sync.Mutex
}

// NewListener creates a new Minecraft listener.
func NewListener(addr string, handler ConnectionHandler) *Listener {
	return &Listener{
		addr:    addr,
		handler: handler,
		closed:  false,
	}
}

// Start begins accepting connections.
func (l *Listener) Start() error {
	listener, err := net.Listen("tcp", l.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", l.addr, err)
	}
	l.listener = listener

	log.Printf("Server listening on %s", l.addr)

	l.wg.Add(1)
	go l.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections.
func (l *Listener) acceptLoop() {
	defer l.wg.Done()

	for {
		l.mu.Lock()
		closed := l.closed
		l.mu.Unlock()

		if closed {
			return
		}

		conn, err := l.listener.Accept()
		if err != nil {
			l.mu.Lock()
			if !l.closed {
				log.Printf("Accept error: %v", err)
			}
			l.mu.Unlock()
			return
		}

		log.Printf("New connection from %s", conn.RemoteAddr())

		// TODO: Handle connection in goroutine pool with rate limiting
		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			wrappedConn := NewConnection(conn)
			l.handler.HandleConnection(wrappedConn)
		}()
	}
}

// Stop closes the listener and waits for all connections to complete.
func (l *Listener) Stop() error {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()

	if l.listener != nil {
		if err := l.listener.Close(); err != nil {
			return err
		}
	}

	l.wg.Wait()
	return nil
}

// Address returns the listener address.
func (l *Listener) Address() string {
	if l.listener != nil {
		return l.listener.Addr().String()
	}
	return l.addr
}

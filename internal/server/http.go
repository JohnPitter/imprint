package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type HTTPServer struct {
	server *http.Server
	port   int
}

func NewHTTPServer(port int, handler http.Handler) *HTTPServer {
	return &HTTPServer{
		port: port,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 5 * time.Minute, // Pipeline endpoints need time for multiple LLM calls
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *HTTPServer) Start() error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}
	log.Printf("[server] HTTP server listening on port %d", s.port)
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[server] HTTP server error: %v", err)
		}
	}()
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	log.Println("[server] Shutting down HTTP server...")
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) Port() int {
	return s.port
}

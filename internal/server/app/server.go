package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

type Server struct {
	server          *http.Server
	shutdownTimeout time.Duration
}

func NewServer(address string, handler http.Handler, shutdownTimeout time.Duration) *Server {
	return &Server{
		server: &http.Server{
			Addr:    address,
			Handler: handler,
		},
		shutdownTimeout: shutdownTimeout,
	}
}

func (s *Server) Start() error {
	// Start server in goroutine
	go func() {
		log.Zap.Info("Starting HTTP server", zap.String("address", s.server.Addr))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Zap.Error("Failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	log.Zap.Info("Gophkeeper server is ready")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Zap.Info("Shutting down server...")

	return s.Stop()
}

func (s *Server) Stop() error {
	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.server.Shutdown(ctx); err != nil {
		log.Zap.Error("Server forced to shutdown", zap.Error(err))
		return err
	}

	log.Zap.Info("Server exited")
	return nil
}

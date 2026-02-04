package utils

import (
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func TestListen(t *testing.T) {
	t.Run("starts server successfully", func(t *testing.T) {
		// Create a test logger
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		// Create a gRPC server
		server := grpc.NewServer()
		defer server.Stop()

		// Find an available port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Failed to find available port: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()

		// Start the server
		errChan, closer, gracefulCloser, err := Listen(server, port, log)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer closer()

		// Verify server is running by trying to connect
		conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer func() { _ = conn.Close() }()

		// Test graceful shutdown
		go func() {
			time.Sleep(50 * time.Millisecond)
			gracefulCloser()
		}()

		// Should get an error when server shuts down
		select {
		case <-errChan:
			// Expected - server shutdown
		case <-time.After(200 * time.Millisecond):
			t.Error("Server did not shut down in time")
		}
	})

	t.Run("fails with invalid port", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		server := grpc.NewServer()
		defer server.Stop()

		// Try to use an invalid port
		_, _, _, err := Listen(server, -1, log)
		if err == nil {
			t.Error("Expected error for invalid port")
		}
	})

	t.Run("fails with port already in use", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		// Find an available port and occupy it
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Failed to find available port: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		defer func() { _ = listener.Close() }()

		server := grpc.NewServer()
		defer server.Stop()

		// Try to use the occupied port
		_, _, _, err = Listen(server, port, log)
		if err == nil {
			t.Error("Expected error for port already in use")
		}
	})
}

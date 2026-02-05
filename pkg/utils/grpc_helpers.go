package utils

import (
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// Listen starts a gRPC server listening on the specified port.
// It returns an error channel, a graceful closer function, a force closer function, and any setup error.
// The error channel receives any errors from the server operation.
//
// Usage:
//
//	server := grpc.NewServer()
//	errChan, closer, gracefulCloser, err := Listen(server, 8080, logger)
//	if err != nil {
//		log.Fatal("Failed to start server", err)
//	}
//	defer closer()
//
//	// Handle server errors
//	go func() {
//		if err := <-errChan; err != nil {
//			log.Error("Server error", logger.StringField("error", err.Error()))
//		}
//	}()
func Listen(s *grpc.Server, listenPort int, log logger.Logger) (chan error, func(), func(), error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", listenPort)) //nolint:noctx // gRPC server manages listener lifecycle
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to listen on port %d: %w", listenPort, err)
	}

	errorChannel := make(chan error, 1)
	go func() {
		log.Info("Starting gRPC server", logger.StringField("address", lis.Addr().String()))
		errorChannel <- s.Serve(lis)
	}()

	gracefulCloser := func() {
		log.Info("Received graceful shutdown signal")
		s.GracefulStop()
	}
	closer := func() {
		log.Info("Received shutdown signal")
		s.Stop()
	}
	return errorChannel, closer, gracefulCloser, nil
}

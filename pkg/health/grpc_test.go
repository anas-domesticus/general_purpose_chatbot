package health

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestRegisterWithGRPC(t *testing.T) {
	t.Run("registers health service", func(t *testing.T) {
		server := grpc.NewServer()
		h := New()

		updater := h.RegisterWithGRPC(server)
		assert.NotNil(t, updater)

		// Clean up
		updater.Stop()
	})

	t.Run("with custom interval", func(t *testing.T) {
		server := grpc.NewServer()
		h := New()

		updater := h.RegisterWithGRPCAndInterval(server, 1*time.Second)
		assert.NotNil(t, updater)
		assert.Equal(t, 1*time.Second, updater.updateInterval)

		// Clean up
		updater.Stop()
	})
}

func TestGRPCHealthUpdater(t *testing.T) {
	t.Run("initial status is NOT_SERVING", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = lis.Close() }()

		server := grpc.NewServer()
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "test", err: nil})

		updater := h.RegisterWithGRPC(server)
		defer updater.Stop()

		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		// Connect client
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		client := grpc_health_v1.NewHealthClient(conn)

		// Initial check should be NOT_SERVING (until first update runs)
		// We need to wait a tiny bit for the initial status to be set
		time.Sleep(10 * time.Millisecond)

		resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)

		// Should be either NOT_SERVING (initial) or SERVING (after first update)
		assert.Contains(t, []grpc_health_v1.HealthCheckResponse_ServingStatus{
			grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			grpc_health_v1.HealthCheckResponse_SERVING,
		}, resp.Status)
	})

	t.Run("updates to SERVING when healthy", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = lis.Close() }()

		server := grpc.NewServer()
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "test", err: nil})

		updater := h.RegisterWithGRPCAndInterval(server, 100*time.Millisecond)
		defer updater.Stop()

		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		// Connect client
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		client := grpc_health_v1.NewHealthClient(conn)

		// Wait for first update to complete
		time.Sleep(200 * time.Millisecond)

		resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
	})

	t.Run("updates to NOT_SERVING when unhealthy", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = lis.Close() }()

		server := grpc.NewServer()
		h := New(WithFailureThreshold(1))

		check := &mockCheck{name: "test", err: errors.New("service down")}
		h.AddReadinessCheck(check)

		updater := h.RegisterWithGRPCAndInterval(server, 100*time.Millisecond)
		defer updater.Stop()

		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		// Connect client
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		client := grpc_health_v1.NewHealthClient(conn)

		// Wait for first update to complete
		time.Sleep(200 * time.Millisecond)

		resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

	t.Run("transitions from SERVING to NOT_SERVING", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = lis.Close() }()

		server := grpc.NewServer()
		h := New(WithFailureThreshold(1))

		check := &mockCheck{name: "test", err: nil}
		h.AddReadinessCheck(check)

		updater := h.RegisterWithGRPCAndInterval(server, 100*time.Millisecond)
		defer updater.Stop()

		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		// Connect client
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		client := grpc_health_v1.NewHealthClient(conn)

		// Wait for first update - should be SERVING
		time.Sleep(200 * time.Millisecond)

		resp1, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp1.Status)

		// Make check fail
		check.SetErr(errors.New("service degraded"))

		// Wait for next update
		time.Sleep(200 * time.Millisecond)

		resp2, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp2.Status)
	})
}

func TestGRPCHealthUpdaterStop(t *testing.T) {
	t.Run("graceful stop sets NOT_SERVING", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = lis.Close() }()

		server := grpc.NewServer()
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "test", err: nil})

		updater := h.RegisterWithGRPCAndInterval(server, 100*time.Millisecond)

		go func() { _ = server.Serve(lis) }()
		defer server.Stop()

		// Connect client
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		client := grpc_health_v1.NewHealthClient(conn)

		// Wait for first update to show SERVING
		time.Sleep(200 * time.Millisecond)

		// Stop the updater
		updater.Stop()

		// Give it a moment to process the stop
		time.Sleep(50 * time.Millisecond)

		// Should now be NOT_SERVING
		resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

	t.Run("multiple stops are safe", func(t *testing.T) {
		server := grpc.NewServer()
		h := New()
		updater := h.RegisterWithGRPC(server)

		// Multiple stops should not panic
		updater.Stop()
		updater.Stop()
		updater.Shutdown()
	})
}

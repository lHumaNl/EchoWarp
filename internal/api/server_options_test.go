package api

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func TestNewAPIServerWithOptions_DefaultOptions_ReturnsServerWithDefaults(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "test-token", logger)

	require.NotNil(t, server)
	assert.Equal(t, node, server.node)
	assert.Equal(t, "127.0.0.1:8080", server.bindAddr)
	assert.Equal(t, "test-token", server.token)
	assert.Nil(t, server.wsAllowedOrigins) // Should be nil, defaults applied in wsOrigins()
	assert.Nil(t, server.corsOrigins)
	assert.False(t, server.enablePprof)
}

func TestNewAPIServerWithOptions_WithAllowedOrigins_SetsCustomOrigins(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	customOrigins := []string{"https://example.com", "https://app.example.com:*"}

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "test-token", logger,
		WithAllowedOrigins(customOrigins),
	)

	require.NotNil(t, server)
	assert.Equal(t, customOrigins, server.wsAllowedOrigins)
	assert.Equal(t, customOrigins, server.wsOrigins())
}

func TestNewAPIServerWithOptions_WithCORSOrigins_SetsCORSOrigins(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	corsOrigins := []string{"https://example.com"}

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "test-token", logger,
		WithCORSOrigins(corsOrigins),
	)

	require.NotNil(t, server)
	assert.Equal(t, corsOrigins, server.corsOrigins)
}

func TestNewAPIServerWithOptions_WithPprof_EnablesPprof(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "test-token", logger,
		WithPprof(true),
	)

	require.NotNil(t, server)
	assert.True(t, server.enablePprof)
}

func TestNewAPIServerWithOptions_MultipleOptions_AppliesAll(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	wsOrigins := []string{"https://ws.example.com"}
	corsOrigins := []string{"https://api.example.com"}

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "test-token", logger,
		WithAllowedOrigins(wsOrigins),
		WithCORSOrigins(corsOrigins),
		WithPprof(true),
	)

	require.NotNil(t, server)
	assert.Equal(t, wsOrigins, server.wsAllowedOrigins)
	assert.Equal(t, corsOrigins, server.corsOrigins)
	assert.True(t, server.enablePprof)
}

func TestNewAPIServerWithOptions_NoOptions_AppliesDefaults(t *testing.T) {
	t.Parallel()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "", logger)

	require.NotNil(t, server)
	// Verify default origins are applied via wsOrigins()
	origins := server.wsOrigins()
	assert.Len(t, origins, 4)
	assert.Contains(t, origins, "http://localhost:*")
	assert.Contains(t, origins, "http://127.0.0.1:*")
	assert.Contains(t, origins, "https://localhost:*")
	assert.Contains(t, origins, "https://127.0.0.1:*")
}

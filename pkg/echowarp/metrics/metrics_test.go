package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_RegisteredWithPrometheus(t *testing.T) {
	assert.NotNil(t, ConnectionsTotal)
	assert.NotNil(t, ConnectionDuration)
	assert.NotNil(t, AudioBytesSent)
	assert.NotNil(t, AudioBytesRecv)
	assert.NotNil(t, AudioLatency)
	assert.NotNil(t, AuthAttempts)
	assert.NotNil(t, RateLimiterRejections)
	assert.NotNil(t, ConnectionPoolSize)
	assert.NotNil(t, ConnectionPoolActive)
}

func TestMetrics_ConnectionsTotal_Inc(t *testing.T) {
	before := testutil.ToFloat64(ConnectionsTotal.WithLabelValues(RoleServer, StatusSuccess))
	ConnectionsTotal.WithLabelValues(RoleServer, StatusSuccess).Inc()
	after := testutil.ToFloat64(ConnectionsTotal.WithLabelValues(RoleServer, StatusSuccess))
	assert.Equal(t, before+1, after)
}

func TestMetrics_AuthAttempts_Labels(t *testing.T) {
	AuthAttempts.WithLabelValues(StatusSuccess).Inc()
	AuthAttempts.WithLabelValues(StatusFailed).Inc()
}

func TestMetrics_AudioBytesSent_Inc(t *testing.T) {
	before := testutil.ToFloat64(AudioBytesSent)
	AudioBytesSent.Inc()
	after := testutil.ToFloat64(AudioBytesSent)
	assert.Equal(t, before+1, after)
}

func TestMetrics_AudioBytesRecv_Inc(t *testing.T) {
	before := testutil.ToFloat64(AudioBytesRecv)
	AudioBytesRecv.Add(1024)
	after := testutil.ToFloat64(AudioBytesRecv)
	assert.Equal(t, before+1024, after)
}

func TestMetrics_RateLimiterRejections_Inc(t *testing.T) {
	before := testutil.ToFloat64(RateLimiterRejections)
	RateLimiterRejections.Inc()
	after := testutil.ToFloat64(RateLimiterRejections)
	assert.Equal(t, before+1, after)
}

func TestMetrics_ConnectionsTotal_MultipleLabels(t *testing.T) {
	ConnectionsTotal.WithLabelValues(RoleClient, StatusSuccess).Inc()
	ConnectionsTotal.WithLabelValues(RoleClient, StatusFailed).Inc()
	ConnectionsTotal.WithLabelValues(RoleServer, StatusFailed).Inc()
}

func TestMetrics_ConnectionPoolSize_IncDec(t *testing.T) {
	before := testutil.ToFloat64(ConnectionPoolSize)
	ConnectionPoolSize.Inc()
	after := testutil.ToFloat64(ConnectionPoolSize)
	assert.Equal(t, before+1, after)

	ConnectionPoolSize.Dec()
	final := testutil.ToFloat64(ConnectionPoolSize)
	assert.Equal(t, before, final)
}

func TestMetrics_ConnectionPoolActive_IncDec(t *testing.T) {
	before := testutil.ToFloat64(ConnectionPoolActive)
	ConnectionPoolActive.Inc()
	after := testutil.ToFloat64(ConnectionPoolActive)
	assert.Equal(t, before+1, after)

	ConnectionPoolActive.Dec()
	final := testutil.ToFloat64(ConnectionPoolActive)
	assert.Equal(t, before, final)
}

func TestMetrics_ConnectionPoolSize_Add(t *testing.T) {
	before := testutil.ToFloat64(ConnectionPoolSize)
	ConnectionPoolSize.Add(5)
	after := testutil.ToFloat64(ConnectionPoolSize)
	assert.Equal(t, before+5, after)
}

func TestMetrics_ConnectionPoolActive_Sub(t *testing.T) {
	ConnectionPoolActive.Add(10)
	before := testutil.ToFloat64(ConnectionPoolActive)
	ConnectionPoolActive.Sub(3)
	after := testutil.ToFloat64(ConnectionPoolActive)
	assert.Equal(t, before-3, after)
}

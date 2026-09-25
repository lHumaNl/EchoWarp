package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

const audioRouteTestPath = "/api/v1/audio/routes"
const audioRouteTestBody = `{"scope":"receive","source":"*","recipient":"self","muted":true}`

type audioRouteAPILifecycle struct{ started chan struct{} }

func (r *audioRouteAPILifecycle) Run(ctx context.Context) error {
	close(r.started)
	<-ctx.Done()
	return nil
}

type audioRouteAPIFake struct {
	*audioRouteAPILifecycle
	set func(context.Context, echowarp.AudioRouteRule) error
	get func(context.Context) (echowarp.AudioRouteState, error)
}

func (r *audioRouteAPIFake) SetAudioRoute(ctx context.Context, rule echowarp.AudioRouteRule) error {
	return r.set(ctx, rule)
}

func (r *audioRouteAPIFake) AudioRoutes(ctx context.Context) (echowarp.AudioRouteState, error) {
	return r.get(ctx)
}

func newAudioRouteAPIFake() *audioRouteAPIFake {
	return &audioRouteAPIFake{
		audioRouteAPILifecycle: &audioRouteAPILifecycle{started: make(chan struct{})},
		set:                    func(context.Context, echowarp.AudioRouteRule) error { return nil },
		get: func(context.Context) (echowarp.AudioRouteState, error) {
			return echowarp.AudioRouteState{SelfID: "client-1"}, nil
		},
	}
}

func newAudioRouteAPINode(t *testing.T, runner echowarp.Runner, mode echowarp.Mode) *echowarp.Node {
	t.Helper()
	factory := func(echowarp.NodeConfig, *slog.Logger, ban.BanManager, *tls.Config, *auth.IPRateLimiter) (echowarp.Runner, error) {
		return runner, nil
	}
	node, err := echowarp.NewNode(echowarp.NodeConfig{Mode: mode}, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)
	if runner != nil {
		require.NoError(t, node.Start(t.Context()))
		t.Cleanup(func() { require.NoError(t, node.Stop()) })
	}
	return node
}

func audioRouteAPIHandler(t *testing.T, runner echowarp.Runner, mode echowarp.Mode, token string) http.Handler {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	node := newAudioRouteAPINode(t, runner, mode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewAPIServerWithOptions(node, "127.0.0.1:0", token, logger)
	s.initMiddleware()
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return s.buildHandlerChain(mux)
}

func audioRouteAPIRequest(ctx context.Context, method, body, token string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, method, audioRouteTestPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func audioRouteAPIResponse(t *testing.T, handler http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, audioRouteAPIRequest(t.Context(), method, body, t.Name()))
	return rr
}

func TestAudioRouteAPIGet(t *testing.T) {
	runner := newAudioRouteAPIFake()
	want := echowarp.AudioRouteState{SelfID: "client-1", Rules: []echowarp.AudioRouteRule{
		{Scope: "receive", Source: "*", Recipient: "client-1", Muted: true},
		{Scope: "admin", Source: "client-2", Recipient: "client-1", Muted: true},
	}}
	runner.get = func(context.Context) (echowarp.AudioRouteState, error) { return want, nil }
	handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
	rr := audioRouteAPIResponse(t, handler, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	var got echowarp.AudioRouteState
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, want, got)
}

func TestAudioRouteAPIGetEmptyArray(t *testing.T) {
	handler := audioRouteAPIHandler(t, newAudioRouteAPIFake(), echowarp.ModeClient, t.Name())
	rr := audioRouteAPIResponse(t, handler, http.MethodGet, "")
	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"self_id":"client-1","rules":[]}`, rr.Body.String())
}

func TestAudioRouteAPIPutExactAcknowledgedRule(t *testing.T) {
	runner := newAudioRouteAPIFake()
	var applied []echowarp.AudioRouteRule
	runner.set = func(_ context.Context, rule echowarp.AudioRouteRule) error {
		applied = append(applied, rule)
		return nil
	}
	handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
	for _, body := range []string{audioRouteTestBody, audioRouteTestBody, strings.Replace(audioRouteTestBody, "true", "false", 1)} {
		rr := audioRouteAPIResponse(t, handler, http.MethodPut, body)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, body, rr.Body.String())
	}
	require.Len(t, applied, 3)
	assert.Equal(t, applied[0], applied[1])
	assert.False(t, applied[2].Muted)
	assert.Equal(t, "*", applied[2].Source)
}

var invalidAudioRouteBodies = []string{
	"", "{", "null", "[]", "{}",
	`{"scope":"receive","source":"*","recipient":"self"}`,
	`{"scope":"receive","source":"*","recipient":"self","muted":null}`,
	`{"scope":"receive","source":"*","recipient":"self","muted":"true"}`,
	`{"scope":"receive","source":"*","recipient":"self","muted":true,"extra":1}`,
	`{"scope":"unknown","source":"*","recipient":"self","muted":true}`,
	`{"scope":"receive","source":"bad id","recipient":"self","muted":true}`,
	`{"scope":"receive","source":null,"recipient":"self","muted":true}`,
	audioRouteTestBody + `{}`, audioRouteTestBody + `null`, audioRouteTestBody + `garbage`,
}

func TestAudioRouteAPIStrictJSON(t *testing.T) {
	runner := newAudioRouteAPIFake()
	runner.set = func(context.Context, echowarp.AudioRouteRule) error {
		t.Error("invalid body reached runner")
		return nil
	}
	handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
	for _, body := range invalidAudioRouteBodies {
		rr := audioRouteAPIResponse(t, handler, http.MethodPut, body)
		assert.Equal(t, http.StatusBadRequest, rr.Code, body)
		assert.Contains(t, rr.Body.String(), `"error"`)
	}
}

func TestAudioRouteAPIBodyLimit(t *testing.T) {
	handler := audioRouteAPIHandler(t, newAudioRouteAPIFake(), echowarp.ModeClient, t.Name())
	for _, body := range []string{
		strings.Repeat(" ", maxRequestBodySize) + audioRouteTestBody,
		audioRouteTestBody + strings.Repeat(" ", maxRequestBodySize),
	} {
		rr := audioRouteAPIResponse(t, handler, http.MethodPut, body)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestAudioRouteAPIPermissions(t *testing.T) {
	handler := audioRouteAPIHandler(t, newAudioRouteAPIFake(), echowarp.ModeClient, t.Name())
	for _, body := range []string{
		`{"scope":"admin","source":"*","recipient":"*","muted":true}`,
		`{"scope":"admin","source":"*","recipient":"*","muted":false}`,
		`{"scope":"receive","source":"*","recipient":"client-2","muted":true}`,
		`{"scope":"send","source":"client-2","recipient":"*","muted":true}`,
	} {
		rr := audioRouteAPIResponse(t, handler, http.MethodPut, body)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	}
}

func TestAudioRouteAPIServerAdmin(t *testing.T) {
	handler := audioRouteAPIHandler(t, newAudioRouteAPIFake(), echowarp.ModeServer, t.Name())
	body := `{"scope":"admin","source":"server","recipient":"*","muted":true}`
	rr := audioRouteAPIResponse(t, handler, http.MethodPut, body)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, body, rr.Body.String())
}

func TestAudioRouteAPIRuntimeErrors(t *testing.T) {
	for _, tc := range audioRouteAPIErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			runner := newAudioRouteAPIFake()
			runner.set = func(context.Context, echowarp.AudioRouteRule) error { return tc.err }
			runner.get = func(context.Context) (echowarp.AudioRouteState, error) { return echowarp.AudioRouteState{}, tc.err }
			handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				rr := audioRouteAPIResponse(t, handler, method, audioRouteTestBody)
				assert.Equal(t, tc.status, rr.Code)
				assert.Contains(t, rr.Body.String(), `"error"`)
			}
		})
	}
}

var audioRouteAPIErrorCases = []struct {
	name   string
	err    error
	status int
}{
	{"validation", ewerrors.NewError(ewerrors.ErrConfigValidation, "unknown room ID"), http.StatusBadRequest},
	{"forbidden", ewerrors.NewError(ewerrors.ErrAuthFailed, "unauthorized rule"), http.StatusForbidden},
	{"stopped", ewerrors.NewError(ewerrors.ErrNotRunning, "stopped"), http.StatusConflict},
	{"timeout", ewerrors.NewError(ewerrors.ErrConnectionTimeout, "ack timed out"), http.StatusGatewayTimeout},
	{"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout},
	{"canceled", context.Canceled, http.StatusRequestTimeout},
	{"internal", errors.New("controller failed"), http.StatusInternalServerError},
}

func TestAudioRouteAPINotRunning(t *testing.T) {
	handler := audioRouteAPIHandler(t, nil, echowarp.ModeClient, t.Name())
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rr := audioRouteAPIResponse(t, handler, method, audioRouteTestBody)
		assert.Equal(t, http.StatusConflict, rr.Code)
	}
}

func TestAudioRouteAPIUnsupported(t *testing.T) {
	runner := &audioRouteAPILifecycle{started: make(chan struct{})}
	handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rr := audioRouteAPIResponse(t, handler, method, audioRouteTestBody)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	}
}

func TestAudioRouteAPIAuthentication(t *testing.T) {
	handler := audioRouteAPIHandler(t, nil, echowarp.ModeClient, t.Name())
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, token := range []string{"", "incorrect"} {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, audioRouteAPIRequest(t.Context(), method, audioRouteTestBody, token))
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		}
	}
}

func TestAudioRouteAPILocalhostPolicy(t *testing.T) {
	handler := audioRouteAPIHandler(t, nil, echowarp.ModeClient, "")
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for addr, status := range map[string]int{"127.0.0.1:1234": http.StatusConflict, "192.0.2.1:1234": http.StatusUnauthorized} {
			req := audioRouteAPIRequest(t.Context(), method, audioRouteTestBody, "")
			req.RemoteAddr = addr
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, status, rr.Code)
		}
	}
}

func TestAudioRouteAPIMethodNotAllowed(t *testing.T) {
	handler := audioRouteAPIHandler(t, nil, echowarp.ModeClient, t.Name())
	rr := audioRouteAPIResponse(t, handler, http.MethodPost, audioRouteTestBody)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAudioRouteAPICancellationDuringAcknowledgment(t *testing.T) {
	entered := make(chan struct{})
	runner := newAudioRouteAPIFake()
	runner.set = func(ctx context.Context, _ echowarp.AudioRouteRule) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}
	handler := audioRouteAPIHandler(t, runner, echowarp.ModeClient, t.Name())
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rr, done := audioRouteAPIAsyncRequest(t, ctx, handler)
	<-entered
	assertAudioRouteAPIWaiting(t, done)
	cancel()
	<-done
	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
}

func audioRouteAPIAsyncRequest(t *testing.T, ctx context.Context, handler http.Handler) (*httptest.ResponseRecorder, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	rr := httptest.NewRecorder()
	req := audioRouteAPIRequest(ctx, http.MethodPut, audioRouteTestBody, t.Name())
	go func() {
		handler.ServeHTTP(rr, req)
		close(done)
	}()
	return rr, done
}

func assertAudioRouteAPIWaiting(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Error("HTTP handler returned before server acknowledgment")
	default:
	}
}

func audioRouteAPIAckRunner(entered chan<- struct{}, ack <-chan struct{}) *audioRouteAPIFake {
	runner := newAudioRouteAPIFake()
	runner.set = func(ctx context.Context, _ echowarp.AudioRouteRule) error {
		close(entered)
		select {
		case <-ack:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return runner
}

func TestAudioRouteAPIWaitsForSuccessfulAcknowledgment(t *testing.T) {
	entered, ack := make(chan struct{}), make(chan struct{})
	handler := audioRouteAPIHandler(t, audioRouteAPIAckRunner(entered, ack), echowarp.ModeClient, t.Name())
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rr, done := audioRouteAPIAsyncRequest(t, ctx, handler)
	<-entered
	assertAudioRouteAPIWaiting(t, done)
	close(ack)
	<-done
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, audioRouteTestBody, rr.Body.String())
}

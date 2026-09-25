# Conference routing API

This API controls directed `source → recipient` restrictions in the current
conference. It is exposed through the optional public `AudioRouteController`
capability and the authenticated daemon HTTP API. Existing pause, mute, device,
participant, and recording endpoints are unchanged.

## Public Go contract

```go
type AudioRouteRule struct {
    Scope     string `json:"scope"`
    Source    string `json:"source"`
    Recipient string `json:"recipient"`
    Muted     bool   `json:"muted"`
}

type AudioRouteState struct {
    SelfID string           `json:"self_id"`
    Rules  []AudioRouteRule `json:"rules"`
}

type AudioRouteController interface {
    SetAudioRoute(context.Context, AudioRouteRule) error
    AudioRoutes(context.Context) (AudioRouteState, error)
}
```

`Node.SetAudioRoute(ctx, rule)` and `Node.AudioRoutes(ctx)` delegate to the running
runner. Neither holds the Node mutex during a callback or remote acknowledgment.
Callbacks receive the caller's context and must honor cancellation and runtime
shutdown. The facade rejects an already-canceled context before calling a runner.
Runners must be concurrency-safe, return caller-owned state snapshots, and handle
calls that race with stopping. Readback normalizes missing rules to an empty slice.

`SetAudioRoute` is **synchronous**: nil means the server applied the rule, including
invalidation of queued packets for that direction, not merely that a command was
queued. A client runner must wait for the correlated server acknowledgment.
This facade cannot make a queue-only runner implementation satisfy that contract.

## Identities, ownership, and precedence

- `scope` is exactly `receive`, `send`, or `admin` (case-sensitive).
- `source` and `recipient` are 1–128 ASCII characters: letters, digits, `_`, `.`,
  `:`, or `-`; alternatively the entire value may be `*`.
- `*` matches all sources or recipients on its side, within the rule's scope.
  Partial wildcards such as `client-*` are invalid.
- `self` is resolved by the runtime. For clients, the server binds it to the
  authenticated connection, never to an identity claimed in a request.
- The public mutation API requires `recipient: "self"` for `receive` and
  `source: "self"` for `send`. Use the marker even if you know your `self_id`.
  Readback may contain resolved IDs. A client cannot alter another participant's
  own restrictions or change admin rules, including removing them.
- `admin` is server-only and accepts arbitrary directed room pairs, including
  wildcards and the reserved `server` identity. Runtime authorization and room
  membership validation remain authoritative for all entry paths.
- `server` is the server's own source. Server monitoring, if available, is a
  separate recipient; these calls do not create a playback device or monitor.
- **Deny wins:** delivery requires no applicable receive, send, or admin denial.
  Existing capture pause and administrative restrictions remain independent.
- `muted: false` removes only the exact `(scope, source, recipient)` rule. It is
  not an allow override. It cannot remove a different wildcard rule or a denial
  in another scope. Repeating the same PUT is idempotent.
- A participant's own audio is never returned for playback, even if every
  explicit rule is removed. Already-delivered audio cannot be recalled.
- Rules and IDs are room/session-scoped, not durable preferences. Do not reuse
  addressed rules blindly after reconnecting to a new session or restarting.
  Unknown IDs and room-sized rule limits are checked by the runtime.

## HTTP contract

Both operations use `/api/v1/audio/routes` and the existing authentication policy:
`Authorization: Bearer <configured token>`, or localhost-only access when no token
is configured. A client daemon's token does **not** grant room administrator rights.
Protect the server daemon token as an administrative credential. Use a trusted
local connection or an appropriately secured HTTPS deployment for remote control.

### GET

Returns `200` with an `AudioRouteState`, preserving separate scoped restrictions:

```json
{
  "self_id": "client-1",
  "rules": [
    {"scope":"receive","source":"*","recipient":"client-1","muted":true},
    {"scope":"admin","source":"client-2","recipient":"client-1","muted":true}
  ]
}
```

An empty rules collection is `[]`, not `null`. This is a runtime snapshot, not a
flattened effective-delivery matrix; local pause and other independent controls
must also be considered when explaining silence.

### PUT

Accepts exactly one rule. All four fields are required, and `muted` must be an
explicit JSON boolean. Unknown fields, malformed JSON, null/missing required
values, and trailing JSON are rejected. Request bodies are bounded to 1 MiB,
including trailing whitespace. Send `Content-Type: application/json`.

After acknowledgment, returns `200` with the exact applied request:

```json
{"scope":"send","source":"self","recipient":"client-2","muted":true}
```

The response intentionally preserves `self`; it is not an additional readback.
Use GET for current resolved state. A concurrent mutation can change that state
after this request's acknowledgment.

### Errors

Errors retain the existing `{"error":"message"}` response shape.

| HTTP | Meaning / underlying error |
|---|---|
| 400 | Invalid JSON, fields, scope, identity syntax, or runtime validation (`ErrConfigValidation`) |
| 401 | Missing/incorrect bearer token, or non-local request to a tokenless API |
| 403 | Invalid ownership, client admin operation, or runtime rejection (`ErrAuthFailed`) |
| 408 | Request context canceled; a disconnected caller may not receive the response |
| 409 | Node or conference runtime not running (`ErrNotRunning`) |
| 413 | Body exceeds 1 MiB |
| 429 | Existing API rate limit exceeded |
| 500 | Runner lacks `AudioRouteController` (`ErrInternalState`), or another runtime failure |
| 504 | Context deadline or acknowledgment timeout (`ErrConnectionTimeout`) |

Cancellation or timeout is not proof that a remote mutation was rolled back.
Read back state or retry the same idempotent rule after reconnecting. Do not
interpret connection loss, timeout, or queue admission as success.

## Client examples

Set `CLIENT_API` to your client daemon URL and `CLIENT_TOKEN` to its configured
token. Replace `client-2` with a current room runtime ID, not a nickname.

```sh
curl --fail-with-body "$CLIENT_API/api/v1/audio/routes" \
  -H "Authorization: Bearer $CLIENT_TOKEN"

client_route() {
  curl --fail-with-body -X PUT "$CLIENT_API/api/v1/audio/routes" \
    -H "Authorization: Bearer $CLIENT_TOKEN" \
    -H 'Content-Type: application/json' --data "$1"
}

# Receive: stop/resume hearing one source (including the server source).
client_route '{"scope":"receive","source":"client-2","recipient":"self","muted":true}'
client_route '{"scope":"receive","source":"client-2","recipient":"self","muted":false}'
client_route '{"scope":"receive","source":"server","recipient":"self","muted":true}'
client_route '{"scope":"receive","source":"server","recipient":"self","muted":false}'

# Receive: stop/resume all incoming delivery in this scope.
client_route '{"scope":"receive","source":"*","recipient":"self","muted":true}'
client_route '{"scope":"receive","source":"*","recipient":"self","muted":false}'

# Send: prevent/restore delivery of your source to one recipient.
client_route '{"scope":"send","source":"self","recipient":"client-2","muted":true}'
client_route '{"scope":"send","source":"self","recipient":"client-2","muted":false}'

# Send: prevent/restore delivery of your source to all recipients.
client_route '{"scope":"send","source":"self","recipient":"*","muted":true}'
client_route '{"scope":"send","source":"self","recipient":"*","muted":false}'
```

Each restore only removes the named restriction; other reasons can still block
delivery. A client PUT with `scope: "admin"`, or another client's ID in the owned
send-source/receive-recipient field, returns `403`.

## Server examples

Set `SERVER_API` and `SERVER_TOKEN` for the server daemon. Admin rules are not
client requests forwarded with elevated rights.

```sh
curl --fail-with-body "$SERVER_API/api/v1/audio/routes" \
  -H "Authorization: Bearer $SERVER_TOKEN"

server_route() {
  curl --fail-with-body -X PUT "$SERVER_API/api/v1/audio/routes" \
    -H "Authorization: Bearer $SERVER_TOKEN" \
    -H 'Content-Type: application/json' --data "$1"
}

# Restrict/restore the server's own source without affecting other sources.
server_route '{"scope":"send","source":"self","recipient":"*","muted":true}'
server_route '{"scope":"send","source":"self","recipient":"*","muted":false}'

# Receive preferences for a server monitor recipient, if the runtime provides one.
# These commands do not enable local playback or return the server's own audio.
server_route '{"scope":"receive","source":"client-1","recipient":"self","muted":true}'
server_route '{"scope":"receive","source":"client-1","recipient":"self","muted":false}'

# Admin: one directed pair.
server_route '{"scope":"admin","source":"client-1","recipient":"client-2","muted":true}'
server_route '{"scope":"admin","source":"client-1","recipient":"client-2","muted":false}'

# Admin: all sources to one recipient.
server_route '{"scope":"admin","source":"*","recipient":"client-2","muted":true}'
server_route '{"scope":"admin","source":"*","recipient":"client-2","muted":false}'

# Admin: one source to all recipients (server is a normal routable source).
server_route '{"scope":"admin","source":"server","recipient":"*","muted":true}'
server_route '{"scope":"admin","source":"server","recipient":"*","muted":false}'

# Admin: all delivery.
server_route '{"scope":"admin","source":"*","recipient":"*","muted":true}'
server_route '{"scope":"admin","source":"*","recipient":"*","muted":false}'
```

## Privacy and recording boundary

These are server-enforced network delivery restrictions, **not end-to-end
encryption (E2EE)**. WebRTC transport encryption does not hide a received source
from the forwarding server. A server administrator may record sources received
by the server; a send/receive/admin delivery restriction does not promise to
exclude that source from recording. Recording is a separate consumer and must
not block forwarding. Do not use this API as a confidentiality boundary against
the server operator.

## Runtime and verification boundary

The public facade and HTTP tests use fake runners, requiring no audio hardware
or live WebRTC sessions. They cover validation, ownership guards, optional
support, lifecycle errors, readback, authentication/registration, bounded strict
JSON, callback lock release, acknowledgment waiting, and cancellation. The app
runtime implements this capability for both server and client. Room tests cover
connection-bound ownership, exact removal, wildcard/deny precedence and targeted
queue invalidation. Real local WebRTC tests cover tone delivery, independent
restriction layers, mono/stereo and source leave/rejoin, without audio hardware.

Both ends must support conference protocol version 1. Directed pair editing is
currently exposed through this API and the public Go facade, not a new TUI matrix
editor. Existing TUI mute/pause and server participant controls remain wired.
The current server has no local conference monitor; receive rules targeting
`server` do not create one. Acoustic echo cancellation and physical-device
verification are separate from network routing correctness.

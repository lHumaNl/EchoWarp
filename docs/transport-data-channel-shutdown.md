# Data-channel shutdown race

## Status

Implemented locally and independently reviewed with a `PASS` verdict (0.96
confidence). This document is the durable handoff for the fix so the
requirements survive context compaction and delegated review.

## Incident

The `master` CI run for release `0.9.2` intermittently panicked on macOS:

```text
panic: send on closed channel
pkg/echowarp/transport.(*WebRTCPeer).handleDataChannelMessage
    pkg/echowarp/transport/peer.go:185
```

The failed job was in `go test -tags nolibopusfile -race -short -timeout 120s
./...`. A rerun passed, which confirms timing sensitivity but does not make the
behavior safe.

## Root cause

Pion invokes data-channel `OnMessage` callbacks asynchronously. Each callback
captures an application message channel. `WebRTCPeer.Close` currently closes
the channels in `dataChannels` before closing the Pion peer connection. A
callback can therefore attempt to send to a channel after `Close` has closed
it.

The race exists in all inbound delivery paths:

- remotely-created generic/control/chat channels handled by
  `handleDataChannelMessage`;
- the creator-side control callback in `CreateControlDataChannel`;
- the creator-side chat callback in `CreateChatDataChannel`;
- late `OnOpen` callbacks that can register state after shutdown has begun.

The likely trigger is `TestWebRTCPeer_ControlDataChannel_Bidirectional`: it
sends a control message, checks only that `SendControl` succeeded, then returns
without waiting for delivery. Deferred cleanup can close the receiver while
Pion is still delivering that message.

## Required behavior

1. `Close` and inbound callback delivery must have one synchronization
   boundary.
2. A callback already delivering when shutdown begins must finish before its
   destination channel is closed.
3. A callback that starts after shutdown begins must return without delivering.
4. Late `OnOpen` callbacks must not repopulate peer state after shutdown.
5. Existing message-channel closure semantics must remain intact for consumers.
6. Generic, control, and chat data channels must use the same guarded delivery
   rule.
7. `Close` must remain safe to call more than once.

The intended minimal design is a peer `closed` state protected by the existing
`mu`. Inbound delivery holds `mu.RLock` through the non-blocking channel send;
`Close` takes `mu.Lock`, marks the peer closed, and closes registered channels
only after active readers have left. Late callbacks acquire the lock, observe
`closed`, and return. Registration paths must check the same state.

## Test requirements

- Strengthen `TestWebRTCPeer_ControlDataChannel_Bidirectional` so it waits for
  the receiver's control message and verifies its type, action, and payload
  before teardown.
- Add deterministic unit coverage proving that delivery after `Close` neither
  panics nor publishes a message.
- Add deterministic coverage for a delivery already in progress while `Close`
  waits, proving the channel is not closed until delivery leaves the guarded
  section.
- Cover the shared delivery behavior used by generic, control, and chat paths.
- Keep the integration tests and run the transport package repeatedly with
  `-race` as supplemental validation.

## Non-solutions

Do not hide the race with sleeps, retries, larger timeouts, `recover`, or by
leaving application channels open forever. Reordering `pc.Close()` alone is
also insufficient unless callback completion is synchronized explicitly.

## Validation

Run at minimum:

```bash
go test -tags nolibopusfile -race ./pkg/echowarp/transport
go test -tags nolibopusfile -race -count=20 ./pkg/echowarp/transport
go test -tags nolibopusfile -race -short -timeout 120s ./...
go vet -tags nolibopusfile ./...
golangci-lint run --build-tags nolibopusfile ./...
```

The implementation is complete only after independent review confirms that the
production lifecycle race is fixed rather than merely making the original test
green.

## Implemented approach

- Added a `closed` lifecycle state guarded by `WebRTCPeer.mu`.
- Routed generic, control, and chat inbound messages through one guarded
  delivery function that retains `mu.RLock` for the complete non-blocking send.
- Made `Close` acquire `mu.Lock`, mark shutdown first, wait for active delivery,
  and then close registered application channels.
- Track registered channels by channel identity rather than label, so duplicate
  WebRTC labels cannot overwrite an older consumer and escape shutdown closure.
- Routed creator-side control/chat `OnOpen` callbacks through the common
  registration path, which rejects and closes late channels after shutdown.
- Strengthened the bidirectional control integration test to verify receipt and
  payload before teardown.
- Added deterministic regressions for post-close callbacks and for `Close`
  waiting on an active delivery guard. The guard test observes a queued writer
  through `RWMutex.TryRLock` instead of assuming scheduler progress after a
  fixed sleep.

Validation completed before independent review:

```text
PASS go test -tags nolibopusfile -race ./pkg/echowarp/transport
PASS go test -tags nolibopusfile -race -count=20 ./pkg/echowarp/transport
PASS go test -tags nolibopusfile -race -short -timeout 120s ./...
PASS go vet -tags nolibopusfile ./...
PASS golangci-lint run --build-tags nolibopusfile ./...
PASS git diff --check
```

Independent concurrency review confirmed that all requirements and remediation
findings are resolved: no remaining source-level blocker was identified within
the data-channel shutdown scope.

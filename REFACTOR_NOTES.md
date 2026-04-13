# Refactor Notes — refactor/021-architecture-cleanup

## [SMELL] DeviceGainControl.IsMuted() semantic change
Previously `DeviceGlobalMute` on a `DeviceGainControl` toggled the same `muted` bool as `DeviceToggleMute`. With the new `VolumeController` interface, `DeviceGlobalMute` now toggles a separate `globalMuted` atomic, and `IsMuted()` returns `muted || globalMuted`. Callers that read `IsMuted()` (e.g. `pipeline.go` gain tap) now observe a composite of both flags. This is correct behavior but differs from the pre-refactor state.
File: `internal/app/device_control.go:31`

## [SMELL] bridgeDeviceCommands now closes dst
Before this refactor, `bridgeDeviceCommands` did NOT `defer close(dst)` while all other bridge functions did. After unifying via `bridgeChanTransform`, it now closes dst on exit. This is consistent with other bridges and safe because the context is shared with the consumer, but it is a behavioral change.
File: `internal/cli/helpers.go:92` (generic), callers at `helpers.go:112-166`

## [POTENTIAL_ISSUE] RecordingMixin embedded publicly
`RecordingMixin` and `StatsMixin` are embedded as exported types in `ServerApp` and `ClientApp`. This means external packages could technically access `app.ServerApp.RecordingMixin.recorder` (unexported) or call `startRecordingInternal` directly. Since all fields and methods are unexported this is safe, but if any field becomes exported in the future it would leak into the public API.
File: `internal/app/server.go:86`, `internal/app/client.go:67`

## [SMELL] recording_adapter.go accesses recorderMu and recorder directly
The `StartRecording`/`StopRecording`/`RecordingStatus` adapter methods on both `ServerApp` and `ClientApp` access `s.recorderMu` and `s.recorder` directly outside of the mixin methods. This bypasses the mixin's encapsulation. Consider adding `lockedRecorder() *audio.ConferenceRecorder` to the mixin, or moving the adapter logic into the mixin itself.
File: `internal/app/recording_adapter.go`

## [SMELL] TUI overlay state structs use exported fields
`KickOverlayState` and `BanOverlayState` have exported fields (`ClientID`, `Reasons`, etc.) despite being used only within the `tui` package. This is intentional for struct literal initialization readability but worth noting.
File: `internal/tui/overlay_state.go`

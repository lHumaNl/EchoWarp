//go:build !darwin

package coreaudio

import "fmt"

// Device holds a CoreAudio device's ID, UID, and name.
type Device struct {
	ID   uint32
	UID  string
	Name string
}

func ListDevices() []Device                         { return nil }
func FindDeviceByName(_ []Device, _ string) *Device { return nil }
func GetDefaultOutputDevice() uint32                { return 0 }
func SetDefaultOutputDevice(_ uint32) error         { return fmt.Errorf("not supported") }
func CleanupOrphaned()                              {}
func DestroyAggregateDevice(_ uint32)               {}

func CreateAggregateDevice(_, _, _, _, _ string) (uint32, error) {
	return 0, fmt.Errorf("CoreAudio aggregate devices are only supported on macOS")
}

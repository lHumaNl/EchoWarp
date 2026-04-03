//go:build darwin

// Package coreaudio provides macOS CoreAudio API wrappers for aggregate device management.
// This is in a separate package because CGo cannot coexist with Go assembly files
// in the same package (the audio package has SIMD assembly for mixing).
package coreaudio

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation

#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

// getDeviceCount returns the number of audio devices on the system.
static UInt32 getDeviceCount() {
    AudioObjectPropertyAddress prop = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    UInt32 size = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &prop, 0, NULL, &size);
    if (status != noErr) return 0;
    return size / sizeof(AudioDeviceID);
}

// getDeviceIDs fills the provided array with audio device IDs.
static OSStatus getDeviceIDs(AudioDeviceID *devices, UInt32 count) {
    AudioObjectPropertyAddress prop = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    UInt32 size = count * sizeof(AudioDeviceID);
    return AudioObjectGetPropertyData(kAudioObjectSystemObject, &prop, 0, NULL, &size, devices);
}

// getDeviceUID returns the UID string for a device. Caller must free the result.
static char* getDeviceUID(AudioDeviceID deviceID) {
    AudioObjectPropertyAddress prop = {
        kAudioDevicePropertyDeviceUID,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    CFStringRef uid = NULL;
    UInt32 size = sizeof(CFStringRef);
    OSStatus status = AudioObjectGetPropertyData(deviceID, &prop, 0, NULL, &size, &uid);
    if (status != noErr || uid == NULL) return NULL;

    CFIndex len = CFStringGetMaximumSizeForEncoding(CFStringGetLength(uid), kCFStringEncodingUTF8) + 1;
    char *buf = (char*)malloc(len);
    if (!CFStringGetCString(uid, buf, len, kCFStringEncodingUTF8)) {
        free(buf);
        CFRelease(uid);
        return NULL;
    }
    CFRelease(uid);
    return buf;
}

// getDeviceName returns the name string for a device. Caller must free the result.
static char* getDeviceName(AudioDeviceID deviceID) {
    AudioObjectPropertyAddress prop = {
        kAudioObjectPropertyName,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    CFStringRef name = NULL;
    UInt32 size = sizeof(CFStringRef);
    OSStatus status = AudioObjectGetPropertyData(deviceID, &prop, 0, NULL, &size, &name);
    if (status != noErr || name == NULL) return NULL;

    CFIndex len = CFStringGetMaximumSizeForEncoding(CFStringGetLength(name), kCFStringEncodingUTF8) + 1;
    char *buf = (char*)malloc(len);
    if (!CFStringGetCString(name, buf, len, kCFStringEncodingUTF8)) {
        free(buf);
        CFRelease(name);
        return NULL;
    }
    CFRelease(name);
    return buf;
}

// getDefaultOutputDevice returns the current default output device ID.
static AudioDeviceID getDefaultOutputDevice() {
    AudioObjectPropertyAddress prop = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    AudioDeviceID deviceID = 0;
    UInt32 size = sizeof(AudioDeviceID);
    OSStatus status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &prop, 0, NULL, &size, &deviceID);
    if (status != noErr) return 0;
    return deviceID;
}

// setDefaultOutputDevice sets the system default output device.
static OSStatus setDefaultOutputDevice(AudioDeviceID deviceID) {
    AudioObjectPropertyAddress prop = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    return AudioObjectSetPropertyData(kAudioObjectSystemObject, &prop, 0, NULL, sizeof(AudioDeviceID), &deviceID);
}

// createAggregateDevice creates a multi-output aggregate device.
static AudioDeviceID createAggregateDevice(const char* uid, const char* name,
                                            const char* masterUID,
                                            const char* subUID1, const char* subUID2) {
    CFMutableDictionaryRef desc = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

    CFStringRef cfUID = CFStringCreateWithCString(NULL, uid, kCFStringEncodingUTF8);
    CFStringRef cfName = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    CFStringRef cfMasterUID = CFStringCreateWithCString(NULL, masterUID, kCFStringEncodingUTF8);

    CFDictionarySetValue(desc, CFSTR(kAudioAggregateDeviceUIDKey), cfUID);
    CFDictionarySetValue(desc, CFSTR(kAudioAggregateDeviceNameKey), cfName);
    CFDictionarySetValue(desc, CFSTR(kAudioAggregateDeviceMasterSubDeviceKey), cfMasterUID);

    int one = 1;
    CFNumberRef cfOne = CFNumberCreate(NULL, kCFNumberIntType, &one);
    CFDictionarySetValue(desc, CFSTR(kAudioAggregateDeviceIsStackedKey), cfOne);

    CFStringRef cfSub1 = CFStringCreateWithCString(NULL, subUID1, kCFStringEncodingUTF8);
    CFStringRef cfSub2 = CFStringCreateWithCString(NULL, subUID2, kCFStringEncodingUTF8);

    CFMutableDictionaryRef sub1Dict = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(sub1Dict, CFSTR(kAudioSubDeviceUIDKey), cfSub1);

    CFMutableDictionaryRef sub2Dict = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(sub2Dict, CFSTR(kAudioSubDeviceUIDKey), cfSub2);

    CFMutableArrayRef subDevices = CFArrayCreateMutable(NULL, 2, &kCFTypeArrayCallBacks);
    CFArrayAppendValue(subDevices, sub1Dict);
    CFArrayAppendValue(subDevices, sub2Dict);
    CFDictionarySetValue(desc, CFSTR(kAudioAggregateDeviceSubDeviceListKey), subDevices);

    AudioDeviceID aggDeviceID = 0;
    OSStatus status = AudioHardwareCreateAggregateDevice(desc, &aggDeviceID);

    CFRelease(subDevices);
    CFRelease(sub2Dict);
    CFRelease(sub1Dict);
    CFRelease(cfSub2);
    CFRelease(cfSub1);
    CFRelease(cfOne);
    CFRelease(cfMasterUID);
    CFRelease(cfName);
    CFRelease(cfUID);
    CFRelease(desc);

    if (status != noErr) return 0;
    return aggDeviceID;
}

// destroyAggregateDevice removes a previously created aggregate device.
static OSStatus destroyAggregateDevice(AudioDeviceID deviceID) {
    return AudioHardwareDestroyAggregateDevice(deviceID);
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

// Device holds a CoreAudio device's ID, UID, and name.
type Device struct {
	ID   uint32
	UID  string
	Name string
}

// ListDevices enumerates all CoreAudio devices on the system.
func ListDevices() []Device {
	count := C.getDeviceCount()
	if count == 0 {
		return nil
	}
	ids := make([]C.AudioDeviceID, count)
	status := C.getDeviceIDs(&ids[0], count)
	if status != 0 {
		return nil
	}

	devices := make([]Device, 0, count)
	for _, id := range ids {
		cUID := C.getDeviceUID(id)
		if cUID == nil {
			continue
		}
		uid := C.GoString(cUID)
		C.free(unsafe.Pointer(cUID))

		cName := C.getDeviceName(id)
		name := ""
		if cName != nil {
			name = C.GoString(cName)
			C.free(unsafe.Pointer(cName))
		}

		devices = append(devices, Device{ID: uint32(id), UID: uid, Name: name})
	}
	return devices
}

// FindDeviceByName finds a CoreAudio device whose name matches (exact first, then substring).
func FindDeviceByName(devices []Device, name string) *Device {
	lower := strings.ToLower(name)
	for i := range devices {
		if strings.EqualFold(devices[i].Name, name) {
			return &devices[i]
		}
	}
	for i := range devices {
		if strings.Contains(strings.ToLower(devices[i].Name), lower) {
			return &devices[i]
		}
	}
	return nil
}

// GetDefaultOutputDevice returns the current default output device ID.
func GetDefaultOutputDevice() uint32 {
	return uint32(C.getDefaultOutputDevice())
}

// SetDefaultOutputDevice sets the system default output device.
func SetDefaultOutputDevice(deviceID uint32) error {
	status := C.setDefaultOutputDevice(C.AudioDeviceID(deviceID))
	if status != 0 {
		return fmt.Errorf("setDefaultOutputDevice failed: status %d", status)
	}
	return nil
}

// CreateAggregateDevice creates a CoreAudio aggregate device (multi-output).
// Returns the new device ID.
func CreateAggregateDevice(uid, name, masterSubDeviceUID, subUID1, subUID2 string) (uint32, error) {
	cUID := C.CString(uid)
	defer C.free(unsafe.Pointer(cUID))
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cMaster := C.CString(masterSubDeviceUID)
	defer C.free(unsafe.Pointer(cMaster))
	cSub1 := C.CString(subUID1)
	defer C.free(unsafe.Pointer(cSub1))
	cSub2 := C.CString(subUID2)
	defer C.free(unsafe.Pointer(cSub2))

	aggID := C.createAggregateDevice(cUID, cName, cMaster, cSub1, cSub2)
	if aggID == 0 {
		return 0, fmt.Errorf("AudioHardwareCreateAggregateDevice failed")
	}
	return uint32(aggID), nil
}

// DestroyAggregateDevice removes a previously created aggregate device.
func DestroyAggregateDevice(deviceID uint32) {
	C.destroyAggregateDevice(C.AudioDeviceID(deviceID))
}

// CleanupOrphaned removes stale EchoWarp aggregate devices from previous sessions.
func CleanupOrphaned() {
	devices := ListDevices()
	for _, d := range devices {
		if strings.HasPrefix(d.UID, "EchoWarp_Loopback_") {
			DestroyAggregateDevice(d.ID)
		}
	}
}

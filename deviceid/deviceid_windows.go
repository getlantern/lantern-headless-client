//go:build windows
// +build windows

package deviceid

import (
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"golang.org/x/sys/windows/registry"
)

const (
	keyPath = `Sofware\\Lantern`
)

// Get returns a unique identifier for this device. The identifier is a random UUID that's stored in the registry
// at HKEY_CURRENT_USERS\Software\Lantern\deviceid. If unable to read/write to the registry, this defaults to the
// old-style device ID derived from MAC address.
func Get(string) string {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE|registry.SET_VALUE|registry.WRITE)
	if err != nil {
		pterm.Error.Printfln("Unable to create registry entry to store deviceID, defaulting to old-style device ID, error: %s", err.Error())
		return OldStyleDeviceID()
	}

	existing, _, err := key.GetStringValue("deviceid")
	if err != nil {
		if err != registry.ErrNotExist {
			pterm.Error.Printfln("Unexpected error reading deviceID, default to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		pterm.Debug.Println("Storing new deviceID")
		_deviceID, err := uuid.NewRandom()
		if err != nil {
			pterm.Error.Printfln("Error generating new deviceID, defaulting to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		deviceID := _deviceID.String()
		err = key.SetStringValue("deviceid", deviceID)
		if err != nil {
			pterm.Error.Printfln("Error storing new deviceID, defaulting to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		return deviceID
	}

	return existing
}

//go:build !windows
// +build !windows

package deviceid

import (
	"github.com/pterm/pterm"
	"os"
	"path/filepath"

	"github.com/getlantern/appdir"
	"github.com/google/uuid"
)

// Get returns a unique identifier for this device. The identifier is a random UUID that's stored on
// disk at $HOME/.lanternsecrets/.deviceid. If unable to read/write to that location, this defaults to the
// old-style device ID derived from MAC address.
func Get() string {
	path := filepath.Join(appdir.InHomeDir(".lanternsecrets"))
	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		pterm.Error.Printfln("Unable to create folder to store deviceID, defaulting to old-style device ID, error: %s", err.Error())
		return OldStyleDeviceID()
	}

	filename := filepath.Join(path, ".deviceid")
	existing, err := os.ReadFile(filename)
	if err != nil {
		pterm.Debug.Println("Storing new deviceID")
		_deviceID, err := uuid.NewRandom()
		if err != nil {
			pterm.Error.Printfln("Error generating new deviceID, defaulting to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		deviceID := _deviceID.String()
		err = os.WriteFile(filename, []byte(deviceID), 0644)
		if err != nil {
			pterm.Error.Printfln("Error storing new deviceID, defaulting to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		return deviceID
	} else {
		return string(existing)
	}
}

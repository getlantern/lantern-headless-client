//go:build !windows
// +build !windows

package deviceid

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/getlantern/appdir"
)

// Get returns a unique identifier for this device. The identifier is a random UUID that's stored on
// disk at `DataPath/deviceid`. If unable to read/write to that location, this defaults to the
// old-style device ID derived from MAC address.
func Get(dataPath string) string {
	filename := filepath.Join(dataPath, "deviceid")
	existing, err := os.ReadFile(filename)
	if err != nil {
		deviceID := readFromHomeDir()
		if deviceID == "" {
			deviceID = newDeviceId()
		}
		pterm.Debug.Println("Storing deviceID")
		err = os.WriteFile(filename, []byte(deviceID), 0644)
		if err != nil {
			pterm.Error.Printfln("Error storing deviceID, defaulting to old-style device ID, error: %s", err.Error())
			return OldStyleDeviceID()
		}
		return deviceID
	} else {
		return string(existing)
	}

}

// Returns a deviceID stored at `$HOME/.lanternsecrets/.deviceid`.
// If the file isn't readable, an empty string is returned.
// This location was used by earlier versions of the client.
func readFromHomeDir() string {
	pterm.Debug.Println("Reading deviceID from home directory")
	filename := filepath.Join(appdir.InHomeDir(".lanternsecrets/.deviceid"))
	deviceID, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}
	pterm.Debug.Println("Found deviceID stored inside home directory")
	return string(deviceID)
}

func newDeviceId() string {
	pterm.Debug.Println("Generating new deviceID")
	deviceID, err := uuid.NewRandom()
	if err != nil {
		pterm.Error.Printfln("Error generating new deviceID, defaulting to old-style device ID, error: %s", err.Error())
		return OldStyleDeviceID()
	}
	return deviceID.String()
}

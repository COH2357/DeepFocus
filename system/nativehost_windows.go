//go:build windows

package system

import (
	"golang.org/x/sys/windows/registry"
)

// SetNativeMessagingRegistration points Chrome and Edge at the native host
// manifest. The manifest itself lists the extension id that may connect.
func SetNativeMessagingRegistration(hostName, manifestPath string) error {
	keys := []string{
		`Software\Google\Chrome\NativeMessagingHosts\` + hostName,
		`Software\Microsoft\Edge\NativeMessagingHosts\` + hostName,
	}
	for _, key := range keys {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, key, registry.WRITE)
		if err != nil {
			return err
		}
		if err := k.SetStringValue("", manifestPath); err != nil {
			k.Close()
			return err
		}
		k.Close()
	}
	return nil
}

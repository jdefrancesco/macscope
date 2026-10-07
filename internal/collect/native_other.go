//go:build !darwin

package collect

import "errors"

// MacOSRunner rejects native macOS collection on unsupported platforms.
func MacOSRunner() (Runner, error) {
	return Runner{}, errors.New("unsupported platform: this command requires macOS")
}

//go:build darwin

package collect

import "os"

// MacOSRunner returns a runner with predictable native-tool output.
func MacOSRunner() (Runner, error) {
	return Runner{Env: append(os.Environ(), "LC_ALL=C")}, nil
}

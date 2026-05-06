//go:build !darwin

package mousemover

// IsAccessibilityGranted always returns true on non-macOS platforms.
func IsAccessibilityGranted() bool {
	return true
}

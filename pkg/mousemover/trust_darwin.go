//go:build darwin

package mousemover

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>
*/
import "C"

// IsAccessibilityGranted checks whether the process has been granted
// accessibility access without triggering the system permission prompt.
func IsAccessibilityGranted() bool {
	return C.AXIsProcessTrusted() != 0
}

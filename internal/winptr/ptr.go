package winptr

import "unsafe"

// Ptr converts a Win32 address into a pointer for the duration of the call.
// The conversion lives in assembly so it stays a single, explicit boundary.
func Ptr(p uintptr) unsafe.Pointer

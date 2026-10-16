package winptr

import (
	"testing"
	"unsafe"
)

func TestPtrReadsTheSameAddress(t *testing.T) {
	n := 7
	p := Ptr(uintptr(unsafe.Pointer(&n)))
	if *(*int)(p) != 7 {
		t.Fatal("address was not preserved")
	}
}

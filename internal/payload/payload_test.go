package payload

import (
	"bytes"
	"testing"
)

func TestAppendExtract(t *testing.T) {
	stub := []byte("stub-bytes")
	app := []byte("MZ-rest-of-program")
	packed := Append(stub, app)
	got, err := Extract(packed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, app) {
		t.Fatalf("got %q", got)
	}
}

func TestRejectsTruncated(t *testing.T) {
	if _, err := Extract([]byte("nope")); err == nil {
		t.Fatal("expected error")
	}
	packed := Append([]byte("stub"), []byte("MZ"))
	if _, err := Extract(packed[:len(packed)-1]); err == nil {
		t.Fatal("truncated package was accepted")
	}
}

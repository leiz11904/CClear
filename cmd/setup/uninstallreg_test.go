package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	regTypeSZ       uint32 = 1
	regTypeExpandSZ uint32 = 2
	regTypeBinary   uint32 = 3
	regTypeDWORD    uint32 = 4
	regTypeMultiSZ  uint32 = 7
	regTypeQWORD    uint32 = 11
)

type memRegValue struct {
	typ  uint32
	data []byte
}

type memRegistry struct {
	keys     map[string]map[string]memRegValue
	openErr  map[string]error
	namesErr map[string]error
	getErr   map[string]error
}

func newMemRegistry() *memRegistry {
	return &memRegistry{
		keys:     map[string]map[string]memRegValue{},
		openErr:  map[string]error{},
		namesErr: map[string]error{},
		getErr:   map[string]error{},
	}
}

func (m *memRegistry) ensureKey(path string) {
	if m.keys[path] == nil {
		m.keys[path] = map[string]memRegValue{}
	}
}

func (m *memRegistry) setValue(path, name string, typ uint32, data []byte) {
	m.ensureKey(path)
	m.keys[path][name] = memRegValue{typ: typ, data: append([]byte(nil), data...)}
}

func (m *memRegistry) keyExists(path string) bool {
	_, ok := m.keys[path]
	return ok
}

func (m *memRegistry) OpenKey(path string) (RegistryKeyView, error) {
	if err := m.openErr[path]; err != nil {
		return nil, err
	}
	values, ok := m.keys[path]
	if !ok {
		return nil, ErrRegistryKeyNotExist
	}
	cp := map[string]memRegValue{}
	for name, item := range values {
		cp[name] = memRegValue{typ: item.typ, data: append([]byte(nil), item.data...)}
	}
	return &memRegistryKey{reg: m, path: path, values: cp}, nil
}

func (m *memRegistry) CreateKey(path string) (RegistryKeyView, error) {
	m.ensureKey(path)
	values := m.keys[path]
	cp := map[string]memRegValue{}
	for name, item := range values {
		cp[name] = memRegValue{typ: item.typ, data: append([]byte(nil), item.data...)}
	}
	return &memRegistryKey{reg: m, path: path, values: cp}, nil
}

func (m *memRegistry) DeleteKey(path string) error {
	if _, ok := m.keys[path]; !ok {
		return ErrRegistryKeyNotExist
	}
	delete(m.keys, path)
	return nil
}

type memRegistryKey struct {
	reg    *memRegistry
	path   string
	values map[string]memRegValue
}

func (k *memRegistryKey) Close() error { return nil }

func (k *memRegistryKey) ValueNames() ([]string, error) {
	if err := k.reg.namesErr[k.path]; err != nil {
		return nil, err
	}
	names := make([]string, 0, len(k.values))
	for name := range k.values {
		names = append(names, name)
	}
	return names, nil
}

func (k *memRegistryKey) GetValue(name string) (uint32, []byte, error) {
	if err := k.reg.getErr[k.path+"\x00"+name]; err != nil {
		return 0, nil, err
	}
	item, ok := k.values[name]
	if !ok {
		return 0, nil, fmt.Errorf("value %q missing", name)
	}
	return item.typ, append([]byte(nil), item.data...), nil
}

func (k *memRegistryKey) SetValue(name string, typ uint32, data []byte) error {
	k.values[name] = memRegValue{typ: typ, data: append([]byte(nil), data...)}
	k.reg.keys[k.path] = k.values
	return nil
}

func (k *memRegistryKey) DeleteValue(name string) error {
	delete(k.values, name)
	k.reg.keys[k.path] = k.values
	return nil
}

func utf16LE(s string) []byte {
	buf := make([]byte, (len(s)+1)*2)
	for i, ch := range s {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(ch))
	}
	return buf
}

func multiSZ(parts ...string) []byte {
	var buf []byte
	for _, part := range parts {
		buf = append(buf, utf16LE(part)...)
	}
	return append(buf, 0, 0)
}

func writeInstallRegistryValues(reg *memRegistry, path string) {
	reg.setValue(path, "DisplayName", regTypeSZ, utf16LE("CClear C盘安全清理"))
	reg.setValue(path, "DisplayVersion", regTypeSZ, utf16LE("9.9.9"))
	reg.setValue(path, "Publisher", regTypeSZ, utf16LE("CClear"))
	reg.setValue(path, "InstallLocation", regTypeSZ, utf16LE(`C:\Program Files\CClear`))
	reg.setValue(path, "EstimatedSize", regTypeDWORD, dwordBytes(42))
	reg.setValue(path, "NoModify", regTypeDWORD, dwordBytes(1))
}

func dwordBytes(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

func qwordBytes(v uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf
}

func TestSnapshotRestoreWhenKeyDidNotExist(t *testing.T) {
	reg := newMemRegistry()
	path := uninstallKey
	snap, err := SnapshotUninstallRegistry(reg, path)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Existed {
		t.Fatal("expected missing key")
	}
	writeInstallRegistryValues(reg, path)
	if !reg.keyExists(path) {
		t.Fatal("install should create key")
	}
	if err := snap.Restore(reg); err != nil {
		t.Fatal(err)
	}
	if reg.keyExists(path) {
		t.Fatal("restore should delete key created during install")
	}
}

func TestRestoreRemovesInstallAddedValues(t *testing.T) {
	reg := newMemRegistry()
	path := uninstallKey
	reg.setValue(path, "DisplayName", regTypeSZ, utf16LE("Legacy App"))
	reg.setValue(path, "LegacyFlag", regTypeDWORD, dwordBytes(7))

	snap, err := SnapshotUninstallRegistry(reg, path)
	if err != nil {
		t.Fatal(err)
	}
	writeInstallRegistryValues(reg, path)
	reg.setValue(path, "FreshInstallOnly", regTypeSZ, utf16LE("remove me"))

	if err := snap.Restore(reg); err != nil {
		t.Fatal(err)
	}
	k, err := reg.OpenKey(path)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	names, err := k.ValueNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 restored values, got %d: %v", len(names), names)
	}
	typ, data, err := k.GetValue("DisplayName")
	if err != nil || typ != regTypeSZ || string(data) != string(utf16LE("Legacy App")) {
		t.Fatal("DisplayName not restored")
	}
	typ, data, err = k.GetValue("LegacyFlag")
	if err != nil || typ != regTypeDWORD || !bytes.Equal(data, dwordBytes(7)) {
		t.Fatal("LegacyFlag not restored")
	}
	if _, _, err := k.GetValue("FreshInstallOnly"); err == nil {
		t.Fatal("install-only value should be gone")
	}
	if _, _, err := k.GetValue("EstimatedSize"); err == nil {
		t.Fatal("install EstimatedSize should be gone")
	}
}

func TestUnknownAndMultiTypeValuesPreserved(t *testing.T) {
	reg := newMemRegistry()
	path := uninstallKey
	reg.setValue(path, "", regTypeSZ, utf16LE("default"))
	reg.setValue(path, "ExpandMe", regTypeExpandSZ, utf16LE("%SystemRoot%\\Temp"))
	reg.setValue(path, "Blob", regTypeBinary, []byte{1, 2, 3, 0xff})
	reg.setValue(path, "Many", regTypeMultiSZ, multiSZ("a", "b"))
	reg.setValue(path, "Big", regTypeQWORD, qwordBytes(0x0123456789abcdef))

	snap, err := SnapshotUninstallRegistry(reg, path)
	if err != nil {
		t.Fatal(err)
	}
	writeInstallRegistryValues(reg, path)
	if err := snap.Restore(reg); err != nil {
		t.Fatal(err)
	}

	k, err := reg.OpenKey(path)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	expect := map[string]memRegValue{
		"":         {regTypeSZ, utf16LE("default")},
		"ExpandMe": {regTypeExpandSZ, utf16LE("%SystemRoot%\\Temp")},
		"Blob":     {regTypeBinary, []byte{1, 2, 3, 0xff}},
		"Many":     {regTypeMultiSZ, multiSZ("a", "b")},
		"Big":      {regTypeQWORD, qwordBytes(0x0123456789abcdef)},
	}
	for name, want := range expect {
		gotTyp, gotData, err := k.GetValue(name)
		if err != nil {
			t.Fatalf("value %q: %v", name, err)
		}
		if gotTyp != want.typ || !bytes.Equal(gotData, want.data) {
			t.Fatalf("value %q mismatch: typ=%d data=%x", name, gotTyp, gotData)
		}
	}
}

func TestSnapshotFailureDoesNotReturnSuccess(t *testing.T) {
	reg := newMemRegistry()
	path := uninstallKey
	reg.setValue(path, "A", regTypeSZ, utf16LE("one"))
	reg.setValue(path, "B", regTypeSZ, utf16LE("two"))
	reg.getErr[path+"\x00"+"B"] = errors.New("read failed")

	_, err := SnapshotUninstallRegistry(reg, path)
	if err == nil {
		t.Fatal("expected snapshot error")
	}
}

func TestSnapshotEnumerateFailure(t *testing.T) {
	reg := newMemRegistry()
	path := uninstallKey
	reg.setValue(path, "A", regTypeSZ, utf16LE("one"))
	reg.namesErr[path] = errors.New("enum failed")

	if _, err := SnapshotUninstallRegistry(reg, path); err == nil {
		t.Fatal("expected enumerate failure")
	}
}

func TestInstallAbortsWhenRegistrySnapshotFails(t *testing.T) {
	body, err := os.ReadFile("install_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	idx := strings.Index(text, "regSnap, err := snapshotUninstallRegistry()")
	if idx < 0 {
		t.Fatal("install must snapshot registry with error handling")
	}
	after := text[idx:]
	if !strings.Contains(after, "if err != nil") || !strings.Contains(after, "_ = tx.Rollback()") {
		t.Fatal("install must stop and roll back files when registry snapshot fails")
	}
}

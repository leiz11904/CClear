package main

import (
	"errors"
	"fmt"
	"sort"
)

const uninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\CClear`

// ErrRegistryKeyNotExist means the registry key path is absent.
var ErrRegistryKeyNotExist = errors.New("registry key does not exist")

// RegValueSnapshot is one registry value with its original type and raw bytes.
type RegValueSnapshot struct {
	Name string
	Type uint32
	Data []byte
}

// UninstallRegSnapshot captures all values under the uninstall key before install changes.
type UninstallRegSnapshot struct {
	Path    string
	Existed bool
	Values  []RegValueSnapshot
}

// RegistryKeyView is a single open registry key used for snapshot and restore.
type RegistryKeyView interface {
	Close() error
	ValueNames() ([]string, error)
	GetValue(name string) (typ uint32, data []byte, err error)
	SetValue(name string, typ uint32, data []byte) error
	DeleteValue(name string) error
}

// RegistryAccess opens, creates, or deletes keys by path.
type RegistryAccess interface {
	OpenKey(path string) (RegistryKeyView, error)
	CreateKey(path string) (RegistryKeyView, error)
	DeleteKey(path string) error
}

// SnapshotUninstallRegistry records every value name, type, and byte payload under path.
func SnapshotUninstallRegistry(reg RegistryAccess, path string) (UninstallRegSnapshot, error) {
	snap := UninstallRegSnapshot{Path: path}
	k, err := reg.OpenKey(path)
	if err != nil {
		if errors.Is(err, ErrRegistryKeyNotExist) {
			return snap, nil
		}
		return snap, fmt.Errorf("open uninstall registry key: %w", err)
	}
	defer k.Close()
	snap.Existed = true
	names, err := k.ValueNames()
	if err != nil {
		return snap, fmt.Errorf("enumerate uninstall registry values: %w", err)
	}
	sort.Strings(names)
	for _, name := range names {
		typ, data, err := k.GetValue(name)
		if err != nil {
			return snap, fmt.Errorf("read uninstall registry value %q: %w", name, err)
		}
		snap.Values = append(snap.Values, RegValueSnapshot{
			Name: name,
			Type: typ,
			Data: append([]byte(nil), data...),
		})
	}
	return snap, nil
}

// Restore puts the key back to the captured state. When the key did not exist, the whole key is removed.
func (snap UninstallRegSnapshot) Restore(reg RegistryAccess) error {
	if !snap.Existed {
		if err := reg.DeleteKey(snap.Path); err != nil && !errors.Is(err, ErrRegistryKeyNotExist) {
			return fmt.Errorf("remove created uninstall registry key: %w", err)
		}
		return nil
	}
	k, err := reg.CreateKey(snap.Path)
	if err != nil {
		return fmt.Errorf("open uninstall registry key for restore: %w", err)
	}
	defer k.Close()
	names, err := k.ValueNames()
	if err != nil {
		return fmt.Errorf("enumerate current uninstall registry values: %w", err)
	}
	for _, name := range names {
		if err := k.DeleteValue(name); err != nil {
			return fmt.Errorf("clear uninstall registry value %q: %w", name, err)
		}
	}
	for _, item := range snap.Values {
		if err := k.SetValue(item.Name, item.Type, item.Data); err != nil {
			return fmt.Errorf("restore uninstall registry value %q: %w", item.Name, err)
		}
	}
	return nil
}

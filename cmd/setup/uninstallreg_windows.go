//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var procRegSetValueExW = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegSetValueExW")

type winRegistryAccess struct {
	root registry.Key
}

func productionRegistry() RegistryAccess {
	return winRegistryAccess{root: registry.LOCAL_MACHINE}
}

func snapshotUninstallRegistry() (UninstallRegSnapshot, error) {
	return SnapshotUninstallRegistry(productionRegistry(), uninstallKey)
}

func (snap UninstallRegSnapshot) restoreUninstallRegistry() error {
	return snap.Restore(productionRegistry())
}

func (w winRegistryAccess) OpenKey(path string) (RegistryKeyView, error) {
	k, err := registry.OpenKey(w.root, path, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil, ErrRegistryKeyNotExist
		}
		return nil, err
	}
	return winRegistryKey{k: k}, nil
}

func (w winRegistryAccess) CreateKey(path string) (RegistryKeyView, error) {
	k, _, err := registry.CreateKey(w.root, path, registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	return winRegistryKey{k: k}, nil
}

func (w winRegistryAccess) DeleteKey(path string) error {
	err := registry.DeleteKey(w.root, path)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return ErrRegistryKeyNotExist
		}
		return err
	}
	return nil
}

type winRegistryKey struct {
	k registry.Key
}

func (w winRegistryKey) Close() error {
	return w.k.Close()
}

func (w winRegistryKey) ValueNames() ([]string, error) {
	return w.k.ReadValueNames(0)
}

func (w winRegistryKey) GetValue(name string) (uint32, []byte, error) {
	n, typ, err := w.k.GetValue(name, nil)
	if err != nil {
		if !errors.Is(err, registry.ErrShortBuffer) {
			return 0, nil, err
		}
		if n > 1<<20 {
			return 0, nil, fmt.Errorf("value %q exceeds size limit", name)
		}
	} else if n == 0 {
		return typ, nil, nil
	}
	buf := make([]byte, n)
	got, typ, err := w.k.GetValue(name, buf)
	if err != nil {
		return 0, nil, err
	}
	return typ, append([]byte(nil), buf[:got]...), nil
}

func (w winRegistryKey) SetValue(name string, typ uint32, data []byte) error {
	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var ptr *byte
	if len(data) > 0 {
		ptr = &data[0]
	}
	return regSetValueEx(syscall.Handle(w.k), pname, 0, typ, ptr, uint32(len(data)))
}

func regSetValueEx(key syscall.Handle, valueName *uint16, reserved uint32, vtype uint32, buf *byte, bufsize uint32) error {
	r0, _, _ := syscall.Syscall6(
		procRegSetValueExW.Addr(),
		6,
		uintptr(key),
		uintptr(unsafe.Pointer(valueName)),
		uintptr(reserved),
		uintptr(vtype),
		uintptr(unsafe.Pointer(buf)),
		uintptr(bufsize),
	)
	if r0 != 0 {
		return syscall.Errno(r0)
	}
	return nil
}

func (w winRegistryKey) DeleteValue(name string) error {
	return w.k.DeleteValue(name)
}

//go:build windows

// Package private stores application secrets using per-user Windows DPAPI.
package private

import (
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"unsafe"
)

func protect(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty protected data")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	var err error
	if decrypt {
		err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	if err != nil {
		return nil, errors.New("Windows could not unlock the private application store")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	result := append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...)
	return result, nil
}

// Save encrypts and atomically replaces one bounded JSON record.
func Save(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > 1024*1024 {
		return errors.New("private record exceeds size limit")
	}
	encrypted, err := protect(raw, false)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".pending-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(encrypted); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	from, _ := windows.UTF16PtrFromString(name)
	to, _ := windows.UTF16PtrFromString(path)
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// Load decrypts one record; corrupt or foreign-user records fail closed.
func Load(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 2*1024*1024 {
		return errors.New("private record exceeds size limit")
	}
	encrypted, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil {
		return err
	}
	if len(encrypted) > 2*1024*1024 {
		return errors.New("private record exceeds size limit")
	}
	raw, err := protect(encrypted, true)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

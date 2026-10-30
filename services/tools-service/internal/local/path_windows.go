//go:build windows

package local

import (
	"syscall"
	"unsafe"
)

const (
	fileReadAttributes      = 0x80
	fileShareRead           = 0x01
	fileShareWrite          = 0x02
	fileShareDelete         = 0x04
	openExisting            = 3
	fileFlagBackupSemantics = 0x02000000
)

var getFinalPathNameByHandleW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// canonical 打开路径并取最终路径。目录联接会被跟着走，不能只看字面上是否还在主目录里。
func canonical(path string) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(
		name,
		fileReadAttributes,
		fileShareRead|fileShareWrite|fileShareDelete,
		nil,
		openExisting,
		fileFlagBackupSemantics,
		0,
	)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)

	buf := make([]uint16, 1024)
	n, err := finalPathName(handle, buf)
	if err != nil {
		return "", err
	}
	if n > uint32(len(buf)) {
		buf = make([]uint16, n)
		n, err = finalPathName(handle, buf)
		if err != nil {
			return "", err
		}
	}
	return trimDevicePrefix(syscall.UTF16ToString(buf[:n])), nil
}

func finalPathName(handle syscall.Handle, buf []uint16) (uint32, error) {
	r, _, callErr := getFinalPathNameByHandleW.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	if r == 0 {
		if callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, syscall.EINVAL
	}
	return uint32(r), nil
}

func trimDevicePrefix(path string) string {
	const unc = `\\?\UNC\`
	const dos = `\\?\`
	if len(path) >= len(unc) && path[:len(unc)] == unc {
		return `\\` + path[len(unc):]
	}
	if len(path) >= len(dos) && path[:len(dos)] == dos {
		return path[len(dos):]
	}
	return path
}

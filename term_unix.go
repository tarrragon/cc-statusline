//go:build !windows

package main

import (
	"syscall"
	"unsafe"
)

func getTermWidth() int {
	type winsize struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, 2, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(ws)))
	if errno != 0 || ws.Col == 0 {
		return 120
	}
	return int(ws.Col)
}

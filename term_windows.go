//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func getTermWidth() int {
	handle, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	if err != nil || handle == syscall.InvalidHandle {
		return 120
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleScreenBufferInfo := kernel32.NewProc("GetConsoleScreenBufferInfo")

	type coord struct {
		X, Y int16
	}
	type smallRect struct {
		Left, Top, Right, Bottom int16
	}
	type consoleScreenBufferInfo struct {
		Size              coord
		CursorPosition    coord
		Attributes        uint16
		Window            smallRect
		MaximumWindowSize coord
	}

	var info consoleScreenBufferInfo
	ret, _, _ := getConsoleScreenBufferInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 120
	}

	width := int(info.Window.Right - info.Window.Left + 1)
	if width <= 0 {
		return 120
	}
	return width
}

//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// enableVirtualTerminalProcessing es el flag de la consola de Windows que
// hace que se interpreten los códigos ANSI.
const enableVirtualTerminalProcessing = 0x0004

// enableVirtualTerminal activa el modo VT de la consola de Windows. Sin esto,
// las consolas antiguas muestran las secuencias de escape tal cual.
//
// Solo afecta a la consola de Windows: si no hay consola, o si ya estaba
// activa, no hace nada.
func enableVirtualTerminal() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	getStdHandle := kernel32.NewProc("GetStdHandle")
	handle, _, _ := getStdHandle.Call(uintptr(int32(syscall.STD_OUTPUT_HANDLE)))
	if handle == 0 {
		return
	}

	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	var mode uint32
	if ret, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); ret == 0 {
		return
	}

	if mode&enableVirtualTerminalProcessing != 0 {
		return
	}

	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	_, _, _ = setConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
}

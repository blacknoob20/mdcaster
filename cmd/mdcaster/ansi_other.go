//go:build !windows

package main

// enableVirtualTerminal no hace nada: los terminales de Unix ya interpretan
// los códigos ANSI.
func enableVirtualTerminal() {}

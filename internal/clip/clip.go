// Package clip abstracts the system clipboard so that it can be faked in tests.
package clip

import "github.com/atotto/clipboard"

// Clipboard is a readable and writable clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(string) error
}

// System uses the system clipboard (xclip, wl-clipboard, pbcopy…).
type System struct{}

func (System) Read() (string, error) { return clipboard.ReadAll() }
func (System) Write(s string) error  { return clipboard.WriteAll(s) }

// Memory is a fake in-memory clipboard.
type Memory struct{ Text string }

func (m *Memory) Read() (string, error) { return m.Text, nil }
func (m *Memory) Write(s string) error  { m.Text = s; return nil }

// Package clip abstrait le presse-papiers système pour pouvoir le simuler en test.
package clip

import "github.com/atotto/clipboard"

// Clipboard est un presse-papiers lisible et inscriptible.
type Clipboard interface {
	Read() (string, error)
	Write(string) error
}

// System utilise le presse-papiers du système (xclip, wl-clipboard, pbcopy…).
type System struct{}

func (System) Read() (string, error) { return clipboard.ReadAll() }
func (System) Write(s string) error  { return clipboard.WriteAll(s) }

// Memory est un presse-papiers factice en mémoire.
type Memory struct{ Text string }

func (m *Memory) Read() (string, error) { return m.Text, nil }
func (m *Memory) Write(s string) error  { m.Text = s; return nil }

// Package i18n provides the translated user-facing strings of etui and the
// selection of the interface language (English by default, French available).
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Lang is an interface language.
type Lang int

const (
	// Auto asks for the language to be detected from the environment.
	Auto Lang = iota
	EN
	FR
)

func (l Lang) String() string {
	switch l {
	case EN:
		return "en"
	case FR:
		return "fr"
	}
	return "auto"
}

// current holds the active language; English is the default.
var current atomic.Int32

func init() { current.Store(int32(EN)) }

// Current returns the active language.
func Current() Lang { return Lang(current.Load()) }

// Set makes l the active language. Auto selects the language detected from
// the process environment.
func Set(l Lang) {
	if l == Auto {
		l = Detect(os.Getenv)
	}
	current.Store(int32(l))
}

// Parse reads a language name such as "en", "fr", "fr_FR.UTF-8" or "fr-CA"
// (case-insensitive). "" and "auto" give Auto. Any other value is an error.
func Parse(s string) (Lang, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "auto":
		return Auto, nil
	}
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexAny(s, "_-"); i >= 0 {
		s = s[:i]
	}
	switch s {
	case "en":
		return EN, nil
	case "fr":
		return FR, nil
	}
	return Auto, fmt.Errorf("unknown language %q", s)
}

// envVars lists the variables consulted by Detect, by decreasing priority.
var envVars = []string{"ETUI_LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"}

// Detect returns the language requested by the environment, English if none.
// The first non-empty variable of ETUI_LANG, LC_ALL, LC_MESSAGES, LANGUAGE and
// LANG decides, as with POSIX locales (so LC_ALL=C gives English even when
// LANG is French). An unrecognised ETUI_LANG is ignored.
func Detect(getenv func(string) string) Lang {
	for _, name := range envVars {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		// LANGUAGE is a colon-separated list; the others hold a single value.
		for _, item := range strings.Split(v, ":") {
			if l, err := Parse(item); err == nil && l != Auto {
				return l
			}
		}
		if name != "ETUI_LANG" {
			return EN
		}
	}
	return EN
}

// T returns the message for key in the active language, formatted with args
// when there are any. A key missing from the active catalogue falls back to
// English, then to the key itself.
func T(key Key, args ...any) string {
	s, ok := catalogs[Current()][key]
	if !ok {
		s, ok = catalogs[EN][key]
		if !ok {
			s = string(key)
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

var catalogs = map[Lang]map[Key]string{EN: en, FR: fr}

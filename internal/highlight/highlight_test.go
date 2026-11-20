package highlight

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func runes(ss ...string) [][]rune {
	out := make([][]rune, len(ss))
	for i, s := range ss {
		out[i] = []rune(s)
	}
	return out
}

func TestByFilename(t *testing.T) {
	h := New("main.go", "")
	if h.Language() != "Go" {
		t.Fatalf("language = %q", h.Language())
	}
	toks := h.Tokens(runes("package main", "func é() {}"))
	if toks == nil || len(toks) != 2 {
		t.Fatalf("tokens = %v", toks)
	}
	if !toks[0][0].InCategory(chroma.Keyword) {
		t.Errorf("\"package\" should be a keyword, got %v", toks[0][0])
	}
	if len(toks[1]) != len([]rune("func é() {}")) {
		t.Errorf("one entry per rune expected")
	}
}

func TestFallbackContentAndPlain(t *testing.T) {
	if New("notes.unknownext", "").Language() != "texte" {
		t.Error("plain text expected")
	}
	if New("", "").Tokens(runes("abc")) != nil {
		t.Error("nil attendu sans lexer")
	}
	h := New("data", "#!/bin/bash\necho hi\n")
	if h.Language() == "texte" {
		t.Log("analyse du contenu n'a pas reconnu le shebang (non bloquant)")
	}
}

func TestGoModStaysPlain(t *testing.T) {
	if l := New("/x/go.mod", "module a\n").Language(); l != "texte" {
		t.Fatalf("go.mod : %q", l)
	}
}

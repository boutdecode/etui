// Package highlight tokenises text with Chroma and returns one token type per rune.
package highlight

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// MaxBytes: above this size highlighting is disabled to keep the editor responsive.
const MaxBytes = 512 * 1024

// Highlighter colours a document with a lexer chosen once and for all.
type Highlighter struct {
	lexer chroma.Lexer
}

// plainNames lists files that Chroma wrongly associates with another language
// (go.mod is taken for AMPL): they stay plain text.
var plainNames = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true}

// New picks the lexer from the file name, then from the content.
// Without a recognised lexer, highlighting is neutral (plain text).
func New(path, content string) *Highlighter {
	var lx chroma.Lexer
	if plainNames[filepath.Base(path)] {
		return &Highlighter{}
	}
	if path != "" {
		lx = lexers.Match(path)
	}
	if lx == nil && content != "" {
		lx = lexers.Analyse(content)
	}
	if lx == nil {
		return &Highlighter{}
	}
	return &Highlighter{lexer: chroma.Coalesce(lx)}
}

// Language returns the name of the detected language.
func (h *Highlighter) Language() string {
	if h == nil || h.lexer == nil {
		return "texte"
	}
	return h.lexer.Config().Name
}

// Tokens returns, for each line, the token type of each rune.
// It returns nil if highlighting is unavailable or too expensive.
func (h *Highlighter) Tokens(lines [][]rune) [][]chroma.TokenType {
	if h == nil || h.lexer == nil {
		return nil
	}
	var sb strings.Builder
	for i, l := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(l))
		if sb.Len() > MaxBytes {
			return nil
		}
	}
	it, err := h.lexer.Tokenise(nil, sb.String())
	if err != nil {
		return nil
	}
	out := make([][]chroma.TokenType, len(lines))
	for i, l := range lines {
		out[i] = make([]chroma.TokenType, len(l))
		for j := range out[i] {
			out[i][j] = chroma.Text
		}
	}
	line, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		for _, r := range tok.Value {
			if r == '\n' {
				line++
				col = 0
				continue
			}
			if line < len(out) && col < len(out[line]) {
				out[line][col] = tok.Type
			}
			col++
		}
	}
	return out
}

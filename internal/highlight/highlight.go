// Package highlight tokenise un texte avec Chroma et renvoie un type de jeton par rune.
package highlight

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// MaxBytes : au-delà, la coloration est désactivée pour garder l'éditeur fluide.
const MaxBytes = 512 * 1024

// Highlighter colore un document avec un lexer choisi une fois pour toutes.
type Highlighter struct {
	lexer chroma.Lexer
}

// plainNames liste des fichiers que Chroma associe à tort à un autre langage
// (go.mod est pris pour de l'AMPL) : ils restent en texte brut.
var plainNames = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true}

// New choisit le lexer d'après le nom de fichier, puis d'après le contenu.
// Sans lexer reconnu, la coloration est neutre (texte brut).
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

// Language renvoie le nom du langage détecté.
func (h *Highlighter) Language() string {
	if h == nil || h.lexer == nil {
		return "texte"
	}
	return h.lexer.Config().Name
}

// Tokens renvoie, pour chaque ligne, le type de jeton de chaque rune.
// Renvoie nil si la coloration est indisponible ou trop coûteuse.
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

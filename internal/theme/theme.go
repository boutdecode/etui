// Package theme définit les thèmes clair et sombre de l'interface et le
// choix du style Chroma associé.
package theme

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// Mode est le réglage demandé par l'utilisateur.
type Mode int

const (
	Auto Mode = iota
	Light
	Dark
)

func (m Mode) String() string {
	switch m {
	case Light:
		return "clair"
	case Dark:
		return "sombre"
	}
	return "auto"
}

// Next fait défiler auto → clair → sombre → auto.
func (m Mode) Next() Mode { return (m + 1) % 3 }

// ParseMode lit la valeur du flag --theme.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return Auto, nil
	case "light", "clair":
		return Light, nil
	case "dark", "sombre":
		return Dark, nil
	}
	return Auto, fmt.Errorf("thème inconnu %q (auto, light ou dark)", s)
}

// Noms des styles Chroma utilisés.
const (
	ChromaDark  = "monokai"
	ChromaLight = "github"
)

// Theme regroupe toutes les couleurs de l'interface.
type Theme struct {
	Dark   bool
	Chroma *chroma.Style

	Bg, Fg          color.Color
	Dim             color.Color
	Border          color.Color
	LineNum         color.Color
	LineNumCur      color.Color
	CurLineBg       color.Color
	SelBg           color.Color
	MatchBg         color.Color
	CurMatchBg      color.Color
	MatchFg         color.Color
	TabBg           color.Color
	TabActiveBg     color.Color
	TabActiveFg     color.Color
	StatusBg        color.Color
	StatusFg        color.Color
	Accent          color.Color
	Error           color.Color
	ExplorerDirFg   color.Color
	ExplorerSelBg   color.Color
	ExplorerSelBgNF color.Color // sélection quand l'explorateur n'a pas le focus
}

// Resolve choisit le thème selon le mode demandé et le fond détecté.
func Resolve(mode Mode, detectedDark bool) Theme {
	switch mode {
	case Light:
		return light()
	case Dark:
		return dark()
	}
	if detectedDark {
		return dark()
	}
	return light()
}

func c(s string) color.Color { return lipgloss.Color(s) }

func dark() Theme {
	return Theme{
		Dark: true, Chroma: styles.Get(ChromaDark),
		Bg: c("#1e1f1c"), Fg: c("#f8f8f2"), Dim: c("#75715e"), Border: c("#49483e"),
		LineNum: c("#75715e"), LineNumCur: c("#c0c0b0"), CurLineBg: c("#2a2b26"),
		SelBg: c("#49483e"), MatchBg: c("#5c5a1e"), CurMatchBg: c("#e6a800"), MatchFg: c("#000000"),
		TabBg: c("#2a2b26"), TabActiveBg: c("#1e1f1c"), TabActiveFg: c("#f8f8f2"),
		StatusBg: c("#3b3c35"), StatusFg: c("#f8f8f2"),
		Accent: c("#66d9ef"), Error: c("#f92672"),
		ExplorerDirFg: c("#66d9ef"), ExplorerSelBg: c("#49483e"), ExplorerSelBgNF: c("#33342d"),
	}
}

func light() Theme {
	return Theme{
		Dark: false, Chroma: styles.Get(ChromaLight),
		Bg: c("#ffffff"), Fg: c("#24292e"), Dim: c("#6a737d"), Border: c("#d1d5da"),
		LineNum: c("#959da5"), LineNumCur: c("#24292e"), CurLineBg: c("#f6f8fa"),
		SelBg: c("#b3d4fc"), MatchBg: c("#fff5b1"), CurMatchBg: c("#ffd33d"), MatchFg: c("#000000"),
		TabBg: c("#eaecef"), TabActiveBg: c("#ffffff"), TabActiveFg: c("#24292e"),
		StatusBg: c("#e1e4e8"), StatusFg: c("#24292e"),
		Accent: c("#0366d6"), Error: c("#cb2431"),
		ExplorerDirFg: c("#0366d6"), ExplorerSelBg: c("#c8e1ff"), ExplorerSelBgNF: c("#e1e4e8"),
	}
}

// TokenStyle convertit un type de jeton Chroma en style lipgloss (premier plan,
// gras, italique, souligné ; jamais de fond, qui est géré par les superpositions).
func (t Theme) TokenStyle(tt chroma.TokenType) lipgloss.Style {
	st := lipgloss.NewStyle()
	if t.Chroma == nil {
		return st
	}
	e := t.Chroma.Get(tt)
	if e.Colour.IsSet() {
		st = st.Foreground(lipgloss.Color(e.Colour.String()))
	}
	if e.Bold == chroma.Yes {
		st = st.Bold(true)
	}
	if e.Italic == chroma.Yes {
		st = st.Italic(true)
	}
	if e.Underline == chroma.Yes {
		st = st.Underline(true)
	}
	return st
}

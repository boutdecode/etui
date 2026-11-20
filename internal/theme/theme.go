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

	"boutdecode/etui/internal/i18n"
)

// Mode est le réglage demandé par l'utilisateur.
type Mode int

const (
	Auto Mode = iota
	Light
	Dark
)

// String returns the language-independent name of the mode.
func (m Mode) String() string {
	switch m {
	case Light:
		return "light"
	case Dark:
		return "dark"
	}
	return "auto"
}

// Label returns the mode name in the active interface language.
func (m Mode) Label() string {
	switch m {
	case Light:
		return i18n.T(i18n.ThemeLight)
	case Dark:
		return i18n.T(i18n.ThemeDark)
	}
	return i18n.T(i18n.ThemeAuto)
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
	return Auto, fmt.Errorf(i18n.T(i18n.ErrUnknownTheme), s)
}

// Noms des styles Chroma utilisés.
const (
	ChromaDark  = "dracula"
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
		Bg: c("#14121f"), Fg: c("#e4e0f5"), Dim: c("#9893b8"), Border: c("#2d2a45"),
		LineNum: c("#8680a8"), LineNumCur: c("#ff5fd2"), CurLineBg: c("#1f1b36"),
		SelBg: c("#2a2548"), MatchBg: c("#4a3f7a"), CurMatchBg: c("#ffb86c"), MatchFg: c("#14121f"),
		TabBg: c("#1b1830"), TabActiveBg: c("#14121f"), TabActiveFg: c("#ffffff"),
		StatusBg: c("#7d56f4"), StatusFg: c("#ffffff"),
		Accent: c("#ff5fd2"), Error: c("#ff5555"),
		ExplorerDirFg: c("#7dd3fc"), ExplorerSelBg: c("#2a2548"), ExplorerSelBgNF: c("#211d3a"),
	}
}

func light() Theme {
	return Theme{
		Dark: false, Chroma: styles.Get(ChromaLight),
		Bg: c("#faf8ff"), Fg: c("#2a2640"), Dim: c("#6b6590"), Border: c("#ddd8ef"),
		LineNum: c("#9a95b8"), LineNumCur: c("#c026a3"), CurLineBg: c("#f1edff"),
		SelBg: c("#d9ccff"), MatchBg: c("#fff0a8"), CurMatchBg: c("#ffc94d"), MatchFg: c("#2a2640"),
		TabBg: c("#efeaff"), TabActiveBg: c("#faf8ff"), TabActiveFg: c("#2a2640"),
		StatusBg: c("#7d56f4"), StatusFg: c("#ffffff"),
		Accent: c("#c026a3"), Error: c("#c62828"),
		ExplorerDirFg: c("#0369a1"), ExplorerSelBg: c("#e2d8ff"), ExplorerSelBgNF: c("#ece6ff"),
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

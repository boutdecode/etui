package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"boutdecode/etui/internal/theme"
)

func newEd(text string) *Editor {
	e := New(1)
	e.buf.setText(text)
	e.SetSize(40, 10)
	return e
}

func key(e *Editor, ks string) {
	e.Update(parseKey(ks))
}

func parseKey(ks string) tea.KeyPressMsg {
	var k tea.Key
	parts := strings.Split(ks, "+")
	name := parts[len(parts)-1]
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "ctrl":
			k.Mod |= tea.ModCtrl
		case "shift":
			k.Mod |= tea.ModShift
		case "alt":
			k.Mod |= tea.ModAlt
		}
	}
	named := map[string]rune{"left": tea.KeyLeft, "right": tea.KeyRight, "up": tea.KeyUp, "down": tea.KeyDown,
		"enter": tea.KeyEnter, "backspace": tea.KeyBackspace, "delete": tea.KeyDelete, "tab": tea.KeyTab,
		"esc": tea.KeyEscape, "home": tea.KeyHome, "end": tea.KeyEnd}
	if r, ok := named[name]; ok {
		k.Code = r
		return tea.KeyPressMsg(k)
	}
	k.Code = []rune(name)[0]
	if k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		k.Text = name
	}
	return tea.KeyPressMsg(k)
}

func typ(e *Editor, s string) {
	for _, r := range s {
		e.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestInsertDeleteAndMultiline(t *testing.T) {
	e := newEd("")
	typ(e, "héllo")
	key(e, "enter")
	typ(e, "wörld")
	if e.Text() != "héllo\nwörld" {
		t.Fatalf("texte = %q", e.Text())
	}
	key(e, "backspace")
	key(e, "home")
	key(e, "backspace") // fusionne les lignes
	if e.Text() != "héllowörl" {
		t.Fatalf("texte = %q", e.Text())
	}
	key(e, "delete")
	if e.Text() != "hélloörl" {
		t.Fatalf("texte = %q", e.Text())
	}
}

func TestAutoIndent(t *testing.T) {
	e := newEd("\tfoo")
	key(e, "end")
	key(e, "enter")
	if e.Text() != "\tfoo\n\t" {
		t.Fatalf("texte = %q", e.Text())
	}
}

func TestSelectionForwardBackward(t *testing.T) {
	e := newEd("abcdef\nghijkl")
	for i := 0; i < 3; i++ {
		key(e, "shift+right")
	}
	if got := e.SelectedText(); got != "abc" {
		t.Fatalf("avant : %q", got)
	}
	key(e, "right") // replie à droite de la sélection
	if _, _, ok := e.Selection(); ok || e.Cursor() != (Pos{0, 3}) {
		t.Fatalf("la sélection devrait être repliée en %v", e.Cursor())
	}
	for i := 0; i < 2; i++ {
		key(e, "shift+left")
	}
	if got := e.SelectedText(); got != "bc" {
		t.Fatalf("arrière : %q", got)
	}
}

func TestSelectionMultilineReplaceAndDelete(t *testing.T) {
	e := newEd("abcdef\nghijkl\nmnop")
	e.SetSelection(Pos{0, 4}, Pos{2, 2}) // avant → arrière géré aussi
	if got := e.SelectedText(); got != "ef\nghijkl\nmn" {
		t.Fatalf("sélection = %q", got)
	}
	typ(e, "X") // remplace la sélection à la frappe
	if e.Text() != "abcdXop" {
		t.Fatalf("après remplacement : %q", e.Text())
	}
	e.SetSelection(Pos{0, 5}, Pos{0, 1}) // sélection à l'envers
	key(e, "backspace")
	if e.Text() != "aop" {
		t.Fatalf("après suppression : %q", e.Text())
	}
	e.SelectAll()
	key(e, "delete")
	if e.Text() != "" {
		t.Fatalf("après tout supprimer : %q", e.Text())
	}
}

func TestSelectLineWordAll(t *testing.T) {
	e := newEd("foo bar\nbaz")
	key(e, "ctrl+d")
	if e.SelectedText() != "foo" {
		t.Fatalf("mot = %q", e.SelectedText())
	}
	key(e, "ctrl+l")
	if e.SelectedText() != "foo bar\n" {
		t.Fatalf("ligne = %q", e.SelectedText())
	}
	key(e, "ctrl+a")
	if e.SelectedText() != "foo bar\nbaz" {
		t.Fatalf("tout = %q", e.SelectedText())
	}
}

func TestWordMoves(t *testing.T) {
	e := newEd("foo_bar  baz.qux")
	key(e, "ctrl+right")
	if e.Cursor().Col != 7 {
		t.Fatalf("col = %d", e.Cursor().Col)
	}
	key(e, "ctrl+right")
	if e.Cursor().Col != 12 {
		t.Fatalf("col = %d", e.Cursor().Col)
	}
	key(e, "ctrl+left")
	if e.Cursor().Col != 9 {
		t.Fatalf("col = %d", e.Cursor().Col)
	}
}

func TestPasteMultiline(t *testing.T) {
	e := newEd("ac")
	key(e, "right")
	e.Update(tea.PasteMsg{Content: "b1\r\nb2\n"})
	if e.Text() != "ab1\nb2\nc" {
		t.Fatalf("texte = %q", e.Text())
	}
	if e.Cursor() != (Pos{2, 0}) {
		t.Fatalf("curseur = %v", e.Cursor())
	}
}

func TestUndoRedoGrouping(t *testing.T) {
	e := newEd("")
	typ(e, "abc")
	key(e, "left") // coupe le groupe
	typ(e, "X")
	if e.Text() != "abXc" {
		t.Fatal(e.Text())
	}
	e.Undo()
	if e.Text() != "abc" {
		t.Fatalf("undo 1 = %q", e.Text())
	}
	e.Undo()
	if e.Text() != "" {
		t.Fatalf("undo 2 (regroupé) = %q", e.Text())
	}
	e.Redo()
	e.Redo()
	if e.Text() != "abXc" {
		t.Fatalf("redo = %q", e.Text())
	}
}

func TestUndoSelectionReplace(t *testing.T) {
	e := newEd("hello world")
	e.SetSelection(Pos{0, 0}, Pos{0, 5})
	typ(e, "bye")
	e.Undo()
	if e.Text() != "hello world" {
		t.Fatalf("après undo : %q", e.Text())
	}
	e.Redo()
	if e.Text() != "bye world" {
		t.Fatalf("après redo : %q", e.Text())
	}
}

func TestUndoBackspaceGroup(t *testing.T) {
	e := newEd("abcdef")
	key(e, "end")
	key(e, "backspace")
	key(e, "backspace")
	key(e, "backspace")
	if e.Text() != "abc" {
		t.Fatal(e.Text())
	}
	e.Undo()
	if e.Text() != "abcdef" {
		t.Fatalf("undo = %q", e.Text())
	}
}

func TestModifiedTracking(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("one\r\ntwo\r\n"), 0o644)
	e, err := Open(1, p)
	if err != nil {
		t.Fatal(err)
	}
	e.SetSize(40, 10)
	if e.Modified() || e.Text() != "one\ntwo\n" {
		t.Fatalf("état initial incorrect : %q", e.Text())
	}
	typ(e, "x")
	if !e.Modified() {
		t.Fatal("devrait être modifié")
	}
	e.Undo()
	if e.Modified() {
		t.Fatal("après undo, ne devrait plus être modifié")
	}
	typ(e, "y")
	if err := e.Save(); err != nil || e.Modified() {
		t.Fatalf("save: %v modified=%v", err, e.Modified())
	}
	typ(e, "z") // ne doit pas fusionner avec la frappe enregistrée
	if !e.Modified() {
		t.Fatal("modifié après save + frappe")
	}
	e.Undo()
	if e.Modified() {
		t.Fatal("undo ramène à l'état enregistré")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "yone\r\ntwo\r\n" {
		t.Fatalf("CRLF non préservé : %q", data)
	}
}

func TestOpenBinaryAndMissing(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "b")
	os.WriteFile(bin, []byte{1, 0, 2}, 0o644)
	if _, err := Open(1, bin); err == nil {
		t.Error("erreur attendue pour un binaire")
	}
	e, err := Open(1, filepath.Join(dir, "new.go"))
	if err != nil || e.Text() != "" || e.Language() != "Go" {
		t.Errorf("nouveau fichier : %v %q %q", err, e.Text(), e.Language())
	}
}

func TestFindAll(t *testing.T) {
	lines := [][]rune{[]rune("aaaa"), []rune("Été été"), []rune("")}
	if got := findAll(lines, []rune("aa"), true); len(got) != 3 {
		t.Errorf("chevauchements : %v", got)
	}
	if got := findAll(lines, []rune("été"), true); len(got) != 1 {
		t.Errorf("casse sensible : %v", got)
	}
	if got := findAll(lines, []rune("été"), false); len(got) != 2 || got[0].Start != 0 || got[1].Start != 4 {
		t.Errorf("casse insensible unicode : %v", got)
	}
	if findAll(lines, nil, true) != nil {
		t.Error("requête vide")
	}
}

func TestSearchNavigationCircular(t *testing.T) {
	e := newEd("foo bar foo\nfoo")
	key(e, "ctrl+f")
	typ(e, "foo")
	if cur, total := e.SearchInfo(); total != 3 || cur != 1 {
		t.Fatalf("info = %d/%d", cur, total)
	}
	for i, want := range []int{2, 3, 1} {
		key(e, "enter")
		if cur, _ := e.SearchInfo(); cur != want {
			t.Fatalf("étape %d : %d, attendu %d", i, cur, want)
		}
	}
	key(e, "shift+enter")
	if cur, _ := e.SearchInfo(); cur != 3 {
		t.Fatalf("précédent circulaire : %d", cur)
	}
	if e.SelectedText() != "foo" {
		t.Fatalf("l'occurrence courante devrait être sélectionnée : %q", e.SelectedText())
	}
	key(e, "alt+c") // sensible à la casse
	key(e, "esc")
	if e.SearchActive() || len(e.Matches()) != 0 {
		t.Fatal("recherche fermée")
	}
}

func TestSearchPrefillAndNone(t *testing.T) {
	e := newEd("alpha beta")
	e.SetSelection(Pos{0, 6}, Pos{0, 10})
	key(e, "ctrl+f")
	if _, total := e.SearchInfo(); total != 1 || string(e.search.query) != "beta" {
		t.Fatalf("préremplissage : %q", string(e.search.query))
	}
	typ(e, "zzz")
	if _, total := e.SearchInfo(); total != 0 {
		t.Fatal("aucune occurrence attendue")
	}
	key(e, "enter")
	if e.TakeNotice() == "" {
		t.Fatal("message attendu")
	}
}

func TestReplace(t *testing.T) {
	e := newEd("a b a b a")
	key(e, "ctrl+r")
	typ(e, "a")
	key(e, "tab")
	typ(e, "XY")
	key(e, "enter") // remplace la courante
	if e.Text() != "XY b a b a" {
		t.Fatalf("texte = %q", e.Text())
	}
	key(e, "alt+a")
	if e.Text() != "XY b XY b XY" {
		t.Fatalf("remplacer tout = %q", e.Text())
	}
}

func render(e *Editor, dark bool) (string, CursorPos) {
	return e.View(theme.Resolve(theme.Auto, dark), true)
}

func TestViewDimensionsWideAndTabs(t *testing.T) {
	e := newEd("日本語\tx\n\tfoo\n短い")
	e.SetSize(20, 6)
	out, cur := render(e, true)
	lines := strings.Split(out, "\n")
	if len(lines) != 6 {
		t.Fatalf("lignes = %d", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 20 {
			t.Errorf("ligne %d : largeur %d", i, w)
		}
	}
	if !cur.Visible || cur.X != e.GutterWidth() || cur.Y != 0 {
		t.Errorf("curseur = %+v", cur)
	}
	// curseur après 3 caractères larges (6 colonnes) puis la tabulation (→ colonne 8)
	e.moveTo(Pos{0, 4}, false, false)
	_, cur = render(e, true)
	if cur.X != e.GutterWidth()+8 {
		t.Errorf("x = %d, attendu %d", cur.X, e.GutterWidth()+8)
	}
}

func TestViewHorizontalScrollAndOverlays(t *testing.T) {
	e := newEd(strings.Repeat("日", 30) + "\nsecond")
	e.SetSize(16, 5)
	e.moveTo(Pos{0, 25}, false, false)
	e.SetSelection(Pos{0, 20}, Pos{1, 3}) // sélection multi-ligne
	key(e, "ctrl+f")
	typ(e, "é")
	for i, l := range strings.Split(func() string { s, _ := render(e, false); return s }(), "\n") {
		if w := lipgloss.Width(l); w != 16 {
			t.Errorf("ligne %d : largeur %d", i, w)
		}
	}
}

func TestHighlightMsgDebounce(t *testing.T) {
	e, err := Open(1, writeTmp(t, "x.go", "package main\n"))
	if err != nil {
		t.Fatal(err)
	}
	e.SetSize(40, 5)
	cmd := e.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if cmd == nil {
		t.Fatal("un recalcul différé est attendu")
	}
	if e.hlVerRun == e.version {
		t.Fatal("pas encore recoloré")
	}
	e.Update(HighlightMsg{ID: e.ID, Version: e.version - 1}) // périmé
	if e.hlVerRun == e.version {
		t.Fatal("message périmé ignoré")
	}
	e.Update(HighlightMsg{ID: e.ID, Version: e.version})
	if e.hlVerRun != e.version {
		t.Fatal("recoloré")
	}
}

func writeTmp(t *testing.T, name, content string) string {
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

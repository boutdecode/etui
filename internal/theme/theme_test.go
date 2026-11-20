package theme

import "testing"

func TestResolve(t *testing.T) {
	cases := []struct {
		mode     Mode
		detected bool
		dark     bool
	}{
		{Auto, true, true}, {Auto, false, false}, {Light, true, false}, {Dark, false, true},
	}
	for _, tc := range cases {
		th := Resolve(tc.mode, tc.detected)
		if th.Dark != tc.dark {
			t.Errorf("Resolve(%v,%v).Dark = %v", tc.mode, tc.detected, th.Dark)
		}
		want := ChromaLight
		if tc.dark {
			want = ChromaDark
		}
		if th.Chroma.Name != want {
			t.Errorf("style chroma = %q, attendu %q", th.Chroma.Name, want)
		}
	}
}

func TestParseModeAndNext(t *testing.T) {
	for in, want := range map[string]Mode{"": Auto, "auto": Auto, "light": Light, "DARK": Dark, "clair": Light} {
		got, err := ParseMode(in)
		if err != nil || got != want {
			t.Errorf("ParseMode(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseMode("x"); err == nil {
		t.Error("erreur attendue")
	}
	if Auto.Next() != Light || Light.Next() != Dark || Dark.Next() != Auto {
		t.Error("cycle incorrect")
	}
}

package main

import (
	"flag"
	"io"
	"os"
	"testing"

	"boutdecode/etui/internal/i18n"
)

func TestMain(m *testing.M) {
	i18n.Set(i18n.EN)
	os.Exit(m.Run())
}

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestResolveLang(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  map[string]string
		want i18n.Lang
	}{
		{"default", nil, nil, i18n.EN},
		{"--lang=fr", []string{"--lang=fr"}, nil, i18n.FR},
		{"-lang fr", []string{"-lang", "fr"}, nil, i18n.FR},
		{"--lang fr", []string{"--lang", "fr"}, nil, i18n.FR},
		{"-lang=fr", []string{"-lang=fr"}, nil, i18n.FR},
		{"--lang without value", []string{"--lang"}, map[string]string{"LANG": "fr_FR.UTF-8"}, i18n.FR},
		{"--lang=xx falls back to env", []string{"--lang=xx"}, map[string]string{"LANG": "fr_FR.UTF-8"}, i18n.FR},
		{"--lang=xx without env", []string{"--lang=xx"}, nil, i18n.EN},
		{"-- stops parsing", []string{"--", "--lang=fr"}, nil, i18n.EN},
		{"after a file stops parsing", []string{"a.txt", "--lang=fr"}, nil, i18n.EN},
		{"before a file", []string{"--lang=fr", "a.txt"}, nil, i18n.FR},
		{"with --theme=dark", []string{"--lang=fr", "--theme=dark"}, nil, i18n.FR},
		{"after --theme dark", []string{"--theme", "dark", "--lang=fr"}, nil, i18n.FR},
		{"with --transparent", []string{"--transparent", "--lang=fr"}, nil, i18n.FR},
		{"--lang=fr_FR.UTF-8", []string{"--lang=fr_FR.UTF-8"}, nil, i18n.FR},
		{"--lang=FR", []string{"--lang=FR"}, nil, i18n.FR},
		{"last one wins", []string{"--lang=fr", "--lang=en"}, nil, i18n.EN},
		{"--lang=en over LANG=fr", []string{"--lang=en"}, map[string]string{"LANG": "fr_FR.UTF-8"}, i18n.EN},
		{"ETUI_LANG over LANG", nil, map[string]string{"ETUI_LANG": "fr", "LANG": "en_US.UTF-8"}, i18n.FR},
		{"--lang over ETUI_LANG", []string{"--lang=en"}, map[string]string{"ETUI_LANG": "fr"}, i18n.EN},
		{"--lang=auto uses the system", []string{"--lang=auto"}, map[string]string{"LANG": "fr_FR.UTF-8"}, i18n.FR},
		{"--lang= uses the system", []string{"--lang="}, map[string]string{"LANG": "fr_FR.UTF-8"}, i18n.FR},
		{"LANG=C", nil, map[string]string{"LANG": "C"}, i18n.EN},
		{"LC_ALL over LANG", nil, map[string]string{"LC_ALL": "C", "LANG": "fr_FR.UTF-8"}, i18n.EN},
	}
	for _, c := range cases {
		if got := resolveLang(c.args, env(c.env)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The value seen by the pre-parsing must be the one the real flag parsing
// validates, for every way of writing the flag.
func TestResolveLangMatchesFlagParsing(t *testing.T) {
	cases := [][]string{
		{"--lang=fr"}, {"-lang=fr"}, {"--lang", "fr"}, {"-lang", "fr"},
		{"--lang=xx"}, {"--lang=en", "--lang=fr"}, {"--theme", "dark", "--lang", "fr"},
		{"--", "--lang=fr"}, {"x", "--lang=fr"}, {"--lang"},
	}
	for _, args := range cases {
		f := newFlags(io.Discard, flag.ContinueOnError)
		_ = f.fs.Parse(args)
		want := i18n.EN
		if l, err := i18n.Parse(*f.lang); err == nil && l != i18n.Auto {
			want = l
		}
		if got := resolveLang(args, env(nil)); got != want {
			t.Errorf("%v: resolveLang = %v, flag parsing = %v", args, got, want)
		}
	}
}

func TestFlagDescriptionsTranslated(t *testing.T) {
	defer i18n.Set(i18n.EN)
	for lang, want := range map[i18n.Lang]string{i18n.EN: "print the version and exit", i18n.FR: "affiche la version et quitte"} {
		i18n.Set(lang)
		f := newFlags(io.Discard, flag.ContinueOnError)
		if got := f.fs.Lookup("version").Usage; got != want {
			t.Errorf("%v: usage = %q, want %q", lang, got, want)
		}
		if f.fs.Lookup("lang") == nil {
			t.Error("--lang flag missing")
		}
	}
}

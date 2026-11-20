// Command etui is a terminal text editor with a file explorer and syntax highlighting.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"boutdecode/etui/internal/app"
	"boutdecode/etui/internal/i18n"
	"boutdecode/etui/internal/theme"
)

// version is injected at build time with -ldflags "-X main.version=...".
var version = "dev"

// cliFlags holds the command-line flags.
type cliFlags struct {
	fs          *flag.FlagSet
	version     *bool
	theme       *string
	lang        *string
	transparent *bool
}

// newFlags declares the flags on a new FlagSet. The descriptions are looked up
// in the active language, so the language must be set before calling it.
func newFlags(output io.Writer, handling flag.ErrorHandling) *cliFlags {
	fs := flag.NewFlagSet("etui", handling)
	fs.SetOutput(output)
	f := &cliFlags{fs: fs}
	f.version = fs.Bool("version", false, i18n.T(i18n.FlagVersion))
	f.theme = fs.String("theme", "auto", i18n.T(i18n.FlagTheme))
	f.lang = fs.String("lang", "auto", i18n.T(i18n.FlagLang))
	f.transparent = fs.Bool("transparent", false, i18n.T(i18n.FlagTransparent))
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), i18n.T(i18n.Usage))
		fs.PrintDefaults()
	}
	return f
}

// resolveLang chooses the interface language: --lang if it is valid and not
// "auto", otherwise the environment (ETUI_LANG, LC_ALL, LC_MESSAGES, LANGUAGE,
// LANG), otherwise English.
//
// It runs before the real flag parsing because the flag descriptions must be
// translated before they are declared. It parses args with a throwaway FlagSet
// so that it follows exactly the rules of package flag (-lang or --lang, "="
// or space, parsing stops at the first non-flag argument or at "--"). Errors
// are ignored here: the real parsing reports them.
func resolveLang(args []string, getenv func(string) string) i18n.Lang {
	f := newFlags(io.Discard, flag.ContinueOnError)
	_ = f.fs.Parse(args)
	if l, err := i18n.Parse(*f.lang); err == nil && l != i18n.Auto {
		return l
	}
	return i18n.Detect(getenv)
}

func main() {
	i18n.Set(resolveLang(os.Args[1:], os.Getenv))

	f := newFlags(os.Stderr, flag.ExitOnError)
	_ = f.fs.Parse(os.Args[1:]) // ExitOnError: exits by itself on failure

	if *f.version {
		fmt.Println("etui", version)
		return
	}

	lang, err := i18n.Parse(*f.lang)
	if err != nil {
		fmt.Fprintln(os.Stderr, "etui :", i18n.T(i18n.ErrUnknownLang, *f.lang))
		os.Exit(2)
	}
	if lang != i18n.Auto {
		i18n.Set(lang)
	}

	mode, err := theme.ParseMode(*f.theme)
	if err != nil {
		fmt.Fprintln(os.Stderr, "etui :", err)
		os.Exit(2)
	}

	opts := app.Options{Dir: ".", Mode: mode, Transparent: *f.transparent}
	for i, a := range f.fs.Args() {
		st, err := os.Stat(a)
		switch {
		case err == nil && st.IsDir():
			if i == 0 {
				opts.Dir = a
			}
		default: // existing file, or a file to create
			if len(opts.Files) == 0 && i == 0 {
				opts.Dir = filepath.Dir(a)
			}
			opts.Files = append(opts.Files, a)
		}
	}

	if _, err := tea.NewProgram(app.New(opts)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "etui :", err)
		os.Exit(1)
	}
}

// Commande etui : éditeur de texte TUI avec explorateur de fichiers et coloration syntaxique.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"boutdecode/etui/internal/app"
	"boutdecode/etui/internal/theme"
)

func main() {
	themeFlag := flag.String("theme", "auto", "thème : auto, light ou dark")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage : etui [--theme=auto|light|dark] [dossier | fichier...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	mode, err := theme.ParseMode(*themeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "etui :", err)
		os.Exit(2)
	}

	opts := app.Options{Dir: ".", Mode: mode}
	for i, a := range flag.Args() {
		st, err := os.Stat(a)
		switch {
		case err == nil && st.IsDir():
			if i == 0 {
				opts.Dir = a
			}
		default: // fichier existant ou à créer
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

# etui

Éditeur de texte en terminal (Bubble Tea v2, Lip Gloss v2, Chroma) avec explorateur de fichiers,
onglets, sélection, presse-papiers, recherche/remplacement et coloration syntaxique.

## Utilisation

```sh
make build            # produit bin/etui (Go ≥ 1.26 requis par Bubble Tea v2 ; GO=... pour changer de SDK)
bin/etui [--theme=auto|light|dark] [--transparent] [dossier | fichier...]
make test
```

## Installation

Télécharger l'archive correspondant à votre système (linux/darwin, amd64/arm64) depuis la page [Releases](../../releases), puis :

```sh
tar xzf etui_<version>_<os>_<arch>.tar.gz
sudo mv etui /usr/local/bin/
etui --version
```

## Publier une version

```sh
git tag v1.2.3
git push origin v1.2.3
```

Le workflow `Release` lance les tests puis GoReleaser, qui publie les binaires, `checksums.txt` et le changelog.

Le thème clair/sombre est détecté depuis le fond du terminal (repli : sombre). `Ctrl+T` fait défiler auto → clair → sombre.
Le fond du thème est appliqué au terminal ; `--transparent` conserve le fond du terminal.

## Raccourcis (F1 dans l'application)

| Action | Touches |
|---|---|
| Enregistrer / fermer l'onglet / quitter | `Ctrl+S` / `Ctrl+W` / `Ctrl+Q` |
| Changer d'onglet | `Alt+←/→`, `Ctrl+PgUp/PgDn`, `Alt+1…9` |
| Focus explorateur ↔ éditeur, afficher/masquer | `Ctrl+B`, `Ctrl+E` |
| Explorateur | `↑↓`, `Entrée`, `←`, `.` (fichiers cachés), `F5`, `/` (rechercher par nom ; `Échap` quitte) |
| Annuler / rétablir | `Ctrl+Z` / `Ctrl+Y` |
| Sélection | `Maj+flèches`, `Ctrl+Maj+←/→`, `Ctrl+A` (tout), `Ctrl+L` (ligne), `Ctrl+D` (mot) |
| Copier / couper / coller | `Ctrl+C` / `Ctrl+X` / `Ctrl+V` (système, repli OSC52 en SSH) |
| Rechercher / remplacer | `Ctrl+F` / `Ctrl+R` ; `Entrée`, `Maj+Entrée`, `Alt+C` (casse), `Tab`, `Alt+A` (tout) |

## Structure

- `internal/editor` : buffer de runes, sélection, historique, recherche, rendu (`view.go` définit l'ordre des superpositions : syntaxe < ligne courante < sélection < recherche).
- `internal/explorer` : arbre à chargement paresseux (`search.go` : recherche asynchrone par nom).
- `internal/highlight` : Chroma → un type de jeton par rune, recalcul différé.
- `internal/theme` : thèmes clair/sombre et styles Chroma associés.
- `internal/app` : modèle racine (disposition, focus, onglets, presse-papiers, confirmations).
clair/sombre
## Limites connues

Pas de souris, pas de retour à la ligne automatique, recherche limitée à une seule ligne,
pas d'« enregistrer sous ».

# Plan: lean into the Charmbracelet ecosystem

Status: P0 (guardrails) and P1 (v2 stack migration) are **done**. P2 onward is
open.
Scope: which `github.com/charmbracelet` repos and libraries apply to MemStation,
what each buys us, and in what order to adopt them.

Current stack (from `go.mod`): `charm.land/bubbletea/v2 v2.1.1`,
`charm.land/bubbles/v2 v2.2.1`, `charm.land/lipgloss/v2 v2.1.0`, plus
`x/ansi` and transitive `colorprofile`/`ultraviolet`. Go directive `1.26.7`.

---

## 1. Bottom line

Three tiers.

**Adopt now (high value, low-to-medium risk).**

| Library | Why this project needs it |
| --- | --- |
| `bubbletea` v2 (`charm.land/bubbletea/v2` v2.1.1) | Declarative `tea.View` (alt-screen, mouse, window title, native progress bar, program status), `KeyPressMsg`, paste messages, keyboard enhancements. Removes `--mouse`/alt-screen plumbing from `internal/cli/tui.go` and fixes esc/alt/shift-tab ambiguities in the timing editor. |
| `lipgloss` v2 (`charm.land/lipgloss/v2` v2.1.0) | Cell compositor (`NewLayer`/`Compositor`) so the help overlay and apply/import modals render **over** the tab instead of replacing it; real `LightDark` adaptive colors (the current `AdaptiveColor` pairs are identical light/dark, i.e. dead code); `image/color` types. |
| `bubbles` v2 (`charm.land/bubbles/v2` v2.2.1) | Required companion of the two above. Unlocks `list`, `filepicker`, `table`, `progress`, `textarea` for the profile picker, import/export paths, history tables and job editing. |
| `fang` v2 (`charm.land/fang/v2` v2.0.1) | Replaces the hand-rolled `flag` tree in `internal/cli` with styled help/errors, `--version` with build metadata, shell completions and man pages. Four subcommands with 3-7 flags each justify it. |
| `log` v2 (`charm.land/log/v2` v2.0.1) | One leveled logger for runner/CLI diagnostics, `--log-file`, `--verbose`, structured run events. Today diagnostics are scattered `fmt.Fprintf(deps.Stderr, ...)` calls. |
| `x/exp/charmtone` (v0.1.1) | Real palette tokens for `internal/style` instead of raw 256-color indexes. |
| `vhs` + `vhs-action` | Tape-driven demo GIFs in the README (timings edit, bench, burn-in). The README already has one screenshot and no recording. |

**Adopt later, on a branch (bigger bite, less urgent).**

| Library | Why |
| --- | --- |
| `glamour` v2 (`charm.land/glamour/v2` v2.0.1) | `mem-station report` renders profile/SPD/history as styled markdown; optionally render `notes/*.md` inside the help overlay. |
| `x/editor` (v0.2.0) | Open burn-in job files in `$EDITOR` through `tea.ExecProcess` (the elevate path already uses it). |
| `filepicker` / `list` / `table` (bubbles v2) | Replace typed-name prompts for save/load/import/export and the hand-maintained history columns. |

**Skip (and why).**

| Library | Verdict |
| --- | --- |
| `x/exp/teatest` | Still pinned to bubbletea v1.3.5; cannot drive a v2 program. Bubble Tea v2 ships `tea.WithWindowSize` + `tea.WithColorProfile` for this purpose, so build the harness there instead. |
| `x/exp/golden` | The existing `internal/uitest` is better for this repo: named goldens per view, exact `w x h` assertions, ANSI stripping. Golden's convention is `testdata/<TestName>.golden` with escaped control codes. Not worth the churn. |
| `harmonica` | Physics animation fights a 500 ms elapsed tick and golden stability. No. |
| `huh` v2 | Forms exist already and are inline; huh is a whole form runner with its own keymap. Only reconsider for a first-run setup wizard. |
| `ultraviolet` | Transitive via bubbletea v2; do not import directly. |
| `glow`, `gum`, `charm`, `sequin`, `freeze`, `x/vt`, `inspo` | Tooling/docs only. `sequin` and `x/vt` are useful when debugging a failing golden; `freeze` can render goldens to PNG in CI. Not dependencies. |
| `skate`, `soft-serve`, `wishlist`, `melt`, `keygen`, `pop`, `mods`, `crush`, `fantasy`, `catwalk`, `confettysh`, `hotdiva2000`, `pi-hyper-provider`, `a2tea`, `go-genai`, `openai-go`, `anthropic-sdk-go`, `sh`, `ssh`, `git-lfs-transfer`, `readme-scribe`, `markscribe`, `soft-serve-action`, `promwish`, `tree-sitter-vhs`, `runway`, `xunicode`, `console`, `meta`, packaging taps | Not applicable: wrong problem domain, AI/LLM, SSH-key tooling, or release plumbing we do not have. |

---

## 2. Full repository matrix

All 57 repos returned by `GET /orgs/charmbracelet/repos` (Nov 2026), plus the
`x/` subpackages this project can actually use.

### 2.1 Core TUI stack

| Repo / module | Latest | Used today | Plan |
| --- | --- | --- | --- |
| `bubbletea` → `charm.land/bubbletea/v2` | v1.3.10 / v2.1.1 | **v2 direct** | done (P1) |
| `lipgloss` → `charm.land/lipgloss/v2` | v1.1.0 / v2.1.0 | **v2 direct** | done (P1); compositor still open |
| `bubbles` → `charm.land/bubbles/v2` | v1.0.0 / v2.2.1 | **v2 direct** (`key`, `textinput`, `viewport`) | done (P1); **P3** adds `list`, `filepicker`, `table`, `textarea` |
| `colorprofile` | v0.4.3 | transitive | **P1** use explicitly (`--no-color`, deterministic tests) |
| `x/ansi` | v0.11.9 | direct | keep |
| `x/cellbuf` | v0.0.15 | transitive | keep (v2 replaces it internally) |
| `x/term` | v0.2.2 | transitive | keep |
| `ultraviolet` | pseudo | transitive | do not import |
| `x/exp/charmtone` | v0.1.1 | no | **P2** theme palette |

### 2.2 Components and CLI

| Repo / module | Latest | Fit |
| --- | --- | --- |
| `bubbles/list` | via v2.2.1 | **P3** profile picker for `ctrl+l` (today: typed name) |
| `bubbles/filepicker` | via v2.2.1 | **P3** import/export path chooser |
| `bubbles/table` | via v2.2.1 | **P3** bench + burn-in history tables only (timings grid stays hand-rolled: group headers, per-cell margin/safety styling) |
| `bubbles/progress` | via v2.2.1 | optional; safer to use `tea.View.ProgressBar` (OSC 9;4) and keep the in-panel ASCII bar golden-stable |
| `bubbles/help`, `key` | via v2.2.1 | `key` already used; keep the custom scrollable help overlay |
| `bubbles/textarea` | via v2.2.1 | **P3** job file editor (today job files are read-only, set by `--job`) |
| `bubbles/spinner`, `stopwatch`, `timer`, `paginator`, `cursor` | via v2.2.1 | no: spinner is derived from the app's elapsed tick on purpose |
| `fang` → `charm.land/fang/v2` | v2.0.1 | **P2** CLI front end (cobra/pflag) |
| `log` → `charm.land/log/v2` | v2.0.1 | **P2** diagnostics, `--log-file`, structured run events |
| `charm` (tool + library) | v0.x | no: wraps fang for a different set of conveniences |
| `huh` → `charm.land/huh/v2` | v2.0.3 | deferred; first-run wizard only |
| `glamour` → `charm.land/glamour/v2` | v2.0.1 | **P4** `report` command, docs pane |
| `harmonica` | v0.2.0 | no |
| `wish`, `promwish` | v2.0.5 / v0.x | no: MemStation is a local terminal tool; no SSH serving surface |
| `skate` | v0.x | no: state is structured profiles/history, not a KV store |

### 2.3 CLI tooling, docs and CI

| Repo | Fit |
| --- | --- |
| `vhs` | **P5** `demo/*.tape` → GIFs for README and PR review |
| `vhs-action` | **P5** regenerate demo GIFs on release |
| `freeze` | optional: PNG of a golden view for issue templates |
| `gum` | optional: `scripts/` helpers only, never a binary dependency |
| `sequin`, `x/vt` | debug aids when a golden mismatch is ANSI soup |
| `inspo`, `bubbletea-app-template`, `wizard-tutorial`, `runway` | reference material |
| `glow` (CLI), `pop`, `mods`, `crush`, `fantasy`, `catwalk`, `confettysh`, `hotdiva2000`, `pi-hyper-provider`, `a2tea` | no |
| `soft-serve`, `soft-serve-action`, `wishlist`, `melt`, `keygen`, `ssh`, `git-lfs-transfer`, `sh` | no |
| `readme-scribe`, `markscribe`, `homebrew-tap`, `scoop-bucket`, `winget-pkgs`, `nur`, `meta`, `.github` | no (or release plumbing for later) |
| `go-genai`, `openai-go`, `anthropic-sdk-go`, `xunicode`, `console`, `tree-sitter-vhs`, `a2tea` | no |

### 2.4 `x/` subpackages worth knowing

| Subpackage | Fit |
| --- | --- |
| `x/ansi` | already used for `Truncate`; keep |
| `x/term` | terminal size/isatty helpers behind bubbletea |
| `x/editor` | **P4** `$EDITOR` for job files |
| `x/exp/teatest` | no: v1-only |
| `x/exp/golden` | no: `internal/uitest` covers it |
| `x/exp/charmtone` | **P2** palette |
| `x/exp/ordered`, `x/exp/slice`, `x/exp/strings`, `x/exp/maps` | only if a helper is genuinely missing |
| `x/json`, `x/etag`, `x/vcr`, `x/vt`, `x/mosaic`, `x/pony`, `x/nerdfont`, `x/termios`, `x/conpty`, `x/input`, `x/powernap`, `x/wcwidth`, `x/paths`-style helpers | no |

---

## 3. Phases

Effort is a single-developer estimate including tests, on top of the existing
`gofmt`/`vet`/`build`/`test` gate.

### P0 — Baseline and guardrails (0.5 day) — **done**

Landed in `2fba2ae`; the pre-P0 commit is tagged `baseline-pre-charm`.

1. ~~Tag the pre-migration commit and confirm the suite is green.~~ Done:
   `baseline-pre-charm` points at `23baa99` (before the task runner).
2. ~~Record current goldens.~~ Done: `make goldens` is idempotent — it rewrites
   every golden with no resulting diff, so any later golden change is a real
   change.
3. ~~Add a task runner so the migration has one command.~~ Done: `make check`
   (gofmt, vet, build, test), plus `race`, `integration`, `goldens`,
   `goldens-check`, `bin`, `tidy`, `clean`, `help`. CI now calls the same
   targets and adds the race suite.
4. Finding: the documented `go test ./... -run TestGolden -update` never worked —
   `-update` is only registered by test binaries that link `internal/uitest`, so
   `./internal/sysinfo` and others abort on an unknown flag. `uitest.Update()`
   now also honours `MEM_STATION_UPDATE_GOLDEN=1`, which is what `make goldens`
   uses; the flag still works when scoped to a package that links the harness.

Done criteria met: clean tree, green `make check`/`make race`/`make
integration`, diffable golden baseline.

### P1 — Bubble Tea v2 / Lip Gloss v2 / Bubbles v2 (2-3 days) — **done**

Landed in `8609f28`. Modules:

```
charm.land/bubbletea/v2  v2.1.1
charm.land/bubbles/v2    v2.2.1
charm.land/lipgloss/v2   v2.1.0
github.com/charmbracelet/colorprofile v0.4.3 (indirect)
```

The v1 modules are gone; `internal/style` has one color system. All screens are
unchanged: the suite passes with **zero golden updates**.

What actually changed, versus the estimate:

| Area | Reality |
| --- | --- |
| key handling | `tea.KeyMsg` → `tea.KeyPressMsg`; printable input is a string, so the timing-field filter rewrites `Key.Text` instead of `Key.Runes` |
| root view | `View() tea.View` + `render()`; `AltScreen`, `WindowTitle`, `MouseMode` declared on the view; tests read `View().Content` |
| tab contract | untouched: `Tab.View(w, h) string` stayed, so the blast radius was the root only |
| program options | `internal/cli/tui.go` is option-free; `--no-color` works because `colorprofile.Detect` reads the `NO_COLOR` that `style.UsePlainOutput` sets (no explicit profile needed) |
| viewport | `viewport.New(WithWidth, WithHeight)`, `SetWidth/SetHeight`, `Width()/Height()`; `LineDown/LineUp` became `ScrollDown/ScrollUp`, which short-circuit at their own end, so the scroll direction is now chosen explicitly |
| textinput | `SetWidth`; the project sets no styles, so the `Styles` struct never came up |
| theme | `AdaptiveColor` deleted rather than shimmed: every token held the same color in both modes, so the theme is plain `color.Color` and `--dark` picks the palette |
| ui panels | **the one real trap**: in lipgloss v2 `Width`/`Height` include border and padding, so `Panel`/`HelpOverlay`/`Modal` ask for the full block size instead of `w-2`/`h-2`. This is what kept the goldens byte-identical |
| tests | `internal/uitest` gained `Key`/`Code`/`Ctrl`/`Wheel` helpers; goldens still strip ANSI, so pinning a color profile was unnecessary |

Verified beyond the unit tests: a pty smoke run paints the shell, sets the
window title, restores the alt screen, exits `0` on `q`, and `--no-color`
suppresses SGR color while keeping bold.

Deliberately deferred to P3 (not "cheap" after all):

- Overlay compositing with `lipgloss.NewLayer`/`Compositor`. It changes the help
  and apply-review goldens by design and needs a dimming decision, so it belongs
  with the visual work, not the mechanical migration.
- Native `View.ProgressBar`: it needs a progress fraction on the tab contract
  (`Tab` only exposes `Running()` today).
- Paste into the profile prompts already works for free (`tea.PasteMsg` reaches
  the text input), so no code was needed.

### P2 — CLI: fang v2 + log v2 + charmtone (1-2 days)

- Replace the `flag.FlagSet` tree in `internal/cli/{cli,tui,info,bench,burnin}.go`
  with one cobra root and four subcommands inside `internal/cli/root.go`, executed
  via `fang.Execute(ctx, root, fang.WithVersion(...), fang.WithTheme(...))`.
- Keep the tested `Run(args []string, deps Deps) int` seam and `Deps` injection;
  `RunE` returns a typed `exitError{code}` that `Run` maps to `ExitOK/Failed/Usage/Sigint`.
- **Flag compatibility shim** (`normalizeArgs`): rewrite documented single-dash
  flags (`-json`, `-imc`, `-out`, `-duration`, `-buffer`, `-workers`, `-preset`,
  `-job`, `-tab`, `-state-dir`, `-no-color`, `-mouse`, `-dark`, `-version`) to
  `--long` before pflag sees them. Golden-test the rewriter. README keeps
  `-json` working and documents `--json` as canonical.
- Swap CLI diagnostics to `charm.land/log/v2` writing to `deps.Stderr`; add
  `--log-file PATH`, `--verbose/-v`, `--quiet/-q`. Runner start/finish events get
  structured fields (`kind`, `duration`, `exit_code`, `log_path`).
- Keep the contract: stdout carries exactly one JSON document under `-json`;
  add a test that asserts that with logging enabled at debug level.
- `internal/style`: source tokens from `x/exp/charmtone` (keep the semantic role
  names; only the values change).
- Free wins: `mem-station completion {bash,zsh,fish,powershell}` and
  `mem-station man` from fang/mango.

Acceptance: `mem-station help`, `mem-station bench --help`, bad-flag errors and
`--version` are styled and exit correctly; all existing `cli_test.go` cases pass
unchanged; new tests cover the shim, log levels and stdout purity.

### P3 — Components: profile list, filepicker, history table, job editor (3-4 days)

| Feature | Files | Component |
| --- | --- | --- |
| `ctrl+l` load profile | `internal/sysinfo/{model,update,view}.go` | `bubbles/v2/list` with a delegate styled by `internal/style` |
| `ctrl+o` import / `ctrl+e` export path | same | `bubbles/v2/filepicker` in a sized modal (`DirAllowed`, hidden toggle) |
| bench + burn-in history | `internal/{benchmark,burnin}/view.go` | `bubbles/v2/table` (data only, no group headers) |
| job file view/edit | `internal/burnin/*` | read-only view + `x/editor` / `tea.ExecProcess` for `$EDITOR`; `textarea` only if in-TUI editing is worth it |
| first run | new | skip unless there is something to configure |

Keep hand-rolled: the timings grid (`sysinfo.editorBody`), the footer
(`ui.Footer`), the scrollable help overlay, and `ui.Progress` inside panels —
all are golden-covered and already correct.

Component theming: v2 components take `Styles` structs; map every style from
`style.ActiveTheme()` so `--dark=false` and `NO_COLOR` stay coherent.

Acceptance: new golden views for the list, filepicker and table at 80x24 and
120x40; keys that type letters no longer leak to global bindings (`Editing()`
and `ModalOpen()` are honoured); `store` round-trips unchanged.

### P4 — Reports: glamour v2 (2-3 days, separate branch)

- `mem-station report [--json|--markdown]`: render the applied profile, SPD,
  IMC availability and recent history as glamour markdown.
- Optional: a `notes` pane in the help overlay rendering the repo docs with
  glamour (offline, embedded via `go:embed`).

Prerequisite: bubbletea v2 / lipgloss v2 (P1), because glamour v2 sits on the
same lipgloss v2 color stack.

Acceptance: `mem-station report --markdown` is readable with `NO_COLOR=1` and
under a 16-color terminal, degrades to plain text when piped, and the JSON form
reuses the existing export schema rather than inventing a second one.

### P5 — Docs, demos and CI (1-2 days)

- `demo/timings.tape`, `demo/bench.tape`, `demo/burnin.tape` for VHS; embed
  GIFs in README next to the existing screenshot.
- `.github/workflows/demo.yml` using `vhs-action` on release tags.
- CI: add `go test ./... -race -count=1`, `go vet -tags integration ./...`,
  and a golden-drift guard (fail if `-update` produced an uncommitted diff).
- README: dependency/requirements note, `--json` canonical flags, new commands,
  and a short "built with Charm" section naming the libs in use.

### P6 — Verification

Extend `notes/verification-plan.md`: the v2 key table (`space`, `shift+tab`,
`esc`, paste), overlay compositing at 80x24/120x40/160x50, `--no-color` +
`NO_COLOR` + `CLICOLOR=0`, `--log-file` content, fang error exit codes, and a
full `ctrl+p`→`ctrl+l`→`ctrl+e` profile round-trip.

Already covered by P1, so the manual matrix no longer needs to prove it:

- The v2 stack builds, renders the same frames (zero golden diffs), and runs
  under a real pty: alt screen enters and restores, the window title is set, `q`
  exits `0`.
- `NO_COLOR=1` suppresses SGR color while keeping bold, through
  `colorprofile.Detect` rather than a custom flag path.

---

## 4. Risks and constraints

1. **Two module paths exist simultaneously** (`github.com/charmbracelet/...`
   and `charm.land/...`). Pin the `charm.land` vanity paths for v2 and remove
   v1 in the same commit, so `go.mod` cannot drift into a mixed stack. Done in
   P1: the v1 modules are gone from `go.mod`.
2. **Go directive bump.** bubbletea v2 needs go 1.26.0. CI uses
   `go-version-file: go.mod`; the local toolchain is already 1.27.3.
3. **Single-dash flags are documented.** fang/pflag will misparse `-json` as a
   shorthand cluster. The `normalizeArgs` shim plus a regression test is
   mandatory, not optional.
4. **Golden churn is expected** from v2's width and color engine. Mitigate with
   ANSI stripping (already in `internal/uitest`), a forced color profile in
   tests, and one reviewed view per commit. Outcome: no churn at all once
   `ui.Panel` asked for the full block size.
5. **teatest cannot validate a v2 program.** Use `tea.WithWindowSize` +
   `tea.WithColorProfile` and drive the root model directly; keep the existing
   message-pushing tests.
6. **Third-party component styling.** `list`, `filepicker`, `table` ship their
   own defaults; without mapping them onto `style.Theme` the TUI will look
   inconsistent under `--dark=false`.

---

## 5. Recommended next slice

P0 and P1 are done. Next is **P2 (fang v2 + log v2 + charmtone)**: it needs no
rendering changes and is the highest visible-value CLI work left. P3's first
sliver should be the overlay compositor, since it is the one v2 feature the
migration had to leave on the table.

---

## 6. Open questions

1. Is breaking `-json` acceptable, or must the single-dash form stay forever?
   (Plan assumes it must stay, via the shim.)
2. Vendor dependencies (`go mod vendor`) now that the module path changes to a
   vanity domain, or rely on the proxy?

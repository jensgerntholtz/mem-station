# Plan: lean into the Charmbracelet ecosystem

Status: P0 (guardrails), P1 (v2 stack migration), P2 (CLI: fang/log/tone) and
P3 (overlay, pickers, progress, $EDITOR) are **done**. P4 onward is open; the one
deliberate omission in P3 is the bubbles/table history migration, with the
measurement recorded there.
Scope: which `github.com/charmbracelet` repos and libraries apply to MemStation,
what each buys us, and in what order to adopt them.

Current stack (from `go.mod`): `charm.land/bubbletea/v2 v2.1.1`,
`charm.land/bubbles/v2 v2.2.1`, `charm.land/lipgloss/v2 v2.1.0`, plus
`charm.land/fang/v2 v2.0.1`, `charm.land/log/v2 v2.0.1`, `charm.land/tone v0.2.0`,
`charm.land/cobra`/`pflag`, plus `x/ansi` and transitive
`colorprofile`/`ultraviolet`. Go directive `1.26.7`.

---

## 1. Bottom line

Three tiers.

**Adopt now (high value, low-to-medium risk).**

| Library | Why this project needs it |
| --- | --- |
| `bubbletea` v2 (`charm.land/bubbletea/v2` v2.1.1) | Declarative `tea.View` (alt-screen, mouse, window title, native progress bar, program status), `KeyPressMsg`, paste messages, keyboard enhancements. Removes `--mouse`/alt-screen plumbing from `internal/cli/tui.go` and fixes esc/alt/shift-tab ambiguities in the timing editor. |
| `lipgloss` v2 (`charm.land/lipgloss/v2` v2.1.0) | Cell compositor (`NewLayer`/`Compositor`) so the help overlay and apply/import modals render **over** the tab instead of replacing it; real `LightDark` adaptive colors (the current `AdaptiveColor` pairs are identical light/dark, i.e. dead code); `image/color` types. |
| `bubbles` v2 (`charm.land/bubbles/v2` v2.2.1) | Required companion of the two above. Unlocks `list`, `filepicker`, `table`, `progress`, `textarea` for the profile picker, import/export paths, history tables and job editing. |
| `fang` v2 (`charm.land/fang/v2` v2.0.1) | Replaces the hand-rolled `flag` tree in `internal/cli` with styled help/errors, `--version` with build metadata, shell completions and man pages. Four subcommands with 3-7 flags each justify it. Done (P2). |
| `log` v2 (`charm.land/log/v2` v2.0.1) | One leveled logger for runner/CLI diagnostics, `--log-file`, `--verbose`, structured run events. Today diagnostics are scattered `fmt.Fprintf(deps.Stderr, ...)` calls. Done (P2). |
| `tone` (`charm.land/tone` v0.2.0) | Real palette tokens for `internal/style` instead of raw 256-color indexes. Replaces the deprecated `x/exp/charmtone`. Done (P2). |
| `vhs` + `vhs-action` | Tape-driven demo GIFs in the README (timings edit, bench, burn-in). The README already has one screenshot and no recording. |

**Adopt later, on a branch (bigger bite, less urgent).**

| Library | Why |
| --- | --- |
| `glamour` v2 (`charm.land/glamour/v2` v2.0.1) | `mem-station report` renders profile/SPD/history as styled markdown; optionally render `notes/*.md` inside the help overlay. |
| `x/editor` (v0.2.0) | Open burn-in job files in `$EDITOR` through `tea.ExecProcess`. Done (P3). |
| `filepicker` / `list` (bubbles v2) | Replace typed-name prompts for load and import. Done (P3). `table` was measured and deliberately skipped — see P3. |

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
| `tone` (`charm.land/tone`) | v0.2.0 | no | done (P2); `x/exp/charmtone` is deprecated in its favour |

### 2.2 Components and CLI

| Repo / module | Latest | Fit |
| --- | --- | --- |
| `bubbles/list` | via v2.2.1 | done (P3): profile picker for `ctrl+l` |
| `bubbles/filepicker` | via v2.2.1 | done (P3): import browser for `ctrl+o` |
| `bubbles/table` | via v2.2.1 | deliberately not adopted: pads cells, highlights row 0, counts the header in `WithHeight`, expands columns. The hand-rolled history keeps per-cell verdict colour and 80x24-fit widths |
| `bubbles/progress` | via v2.2.1 | not adopted; `tea.View.ProgressBar` (OSC 9;4) is used instead, keeping the in-panel ASCII bar golden-stable |
| `bubbles/help`, `key` | via v2.2.1 | `key` already used; keep the custom scrollable help overlay |
| `bubbles/textarea` | via v2.2.1 | not adopted: `$EDITOR` via `x/editor` edits job files better |
| `bubbles/spinner`, `stopwatch`, `timer`, `paginator`, `cursor` | via v2.2.1 | no: spinner is derived from the app's elapsed tick on purpose |
| `fang` → `charm.land/fang/v2` | v2.0.1 | done: cobra tree, styled help/errors, completions, man pages |
| `log` → `charm.land/log/v2` | v2.0.1 | done: `--log-file`, `-v`/`-q`, structured run events |
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
| `x/editor` | done (P3): `ctrl+e` edits the selected job file |
| `x/exp/teatest` | no: v1-only |
| `x/exp/golden` | no: `internal/uitest` covers it |
| `x/exp/charmtone` | deprecated: use `charm.land/tone` |
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

### P2 — CLI: fang v2 + log v2 + tone (1-2 days) — **done**

Landed in the `cli: …` commit. Modules added: `charm.land/fang/v2 v2.0.1`,
`charm.land/log/v2 v2.0.1`, `charm.land/tone v0.2.0`.

What landed, versus the estimate:

- **cobra + fang**: `internal/cli/cli.go` builds the tree (root, `tui`, `info`,
  `bench`, `burnin`, `version`) and runs it through `fang.Execute`. The root
  command keeps the launcher flags, so `mem-station --tab=burn-in` and
  `mem-station tui --tab=burn-in` behave the same. `internal/cli/root.go` was not
  needed; the tree lives next to `Run`.
- **Exit codes**: `Run(args, deps) int` and the `Deps` injection are unchanged.
  Commands return a typed `exitError{code, err, silent}`: `usageErr` → 2,
  `fail` → 1 with fang printing the message, and `exitCodeErr(runCode)` → a
  silent 1/130 because stdout already carries the verdict. Any other error from
  cobra (unknown command, unknown flag, wrong arg count) maps to 2.
- **Flag shim**: `normalizeArgs` rewrites the documented single-dash long flags
  (`-json`, `-duration`, `-out`, `-preset`, `-job`, `-tab`, `-no-color`, ...) to
  `--long` before pflag parses. Shorthands (`-v`, `-q`, `-h`), `--`-prefixed
  flags and everything after a bare `--` are untouched. Unit-tested, and the
  end-to-end test asserts `-json` and `--json` produce byte-identical output.
- **log v2**: `Deps.Log` is a leveled logger on stderr with the `mem-station`
  prefix. `--log-file` tees stderr to a `0600` file (opened by
  `PersistentPreRunE`, closed by `Run`), `-v` selects debug, `-q` errors only.
  Bench/burn-in start and finish log structured fields (`duration`, `exit_code`,
  `read_mbs`, `log`). The stdout contract is unchanged: `-json -v` still writes
  exactly one JSON document, which is now covered by a test.
- **tone, not charmtone**: `github.com/charmbracelet/x/exp/charmtone` is
  deprecated in favour of `charm.land/tone`. Every theme token is now a
  `tone.Key` (which satisfies `color.Color`), so the TUI renders truecolor and
  Bubble Tea downsamples to the terminal profile. The `solid()` 256-color helper
  is gone.
- **Free wins**: `mem-station completion {bash,zsh,fish,powershell}`,
  `mem-station man`, per-command `--help`, and `mem-station version`.

Two intentional behaviour changes, both worth knowing before reviewing a diff:

1. Help output is fang's (`USAGE`, `COMMANDS`, `FLAGS`), not the hand-written
   block. The test assertion moved from the literal `Usage:` to a
   case-insensitive match.
2. fang capitalizes the first word of an error, so `cli_test.go` asserts wording
   case-insensitively (`containsFold`) rather than exact lowercase.

Verified: `make check`/`race`/`integration` green; a pty run paints with tone
colors (`#f7f6fb`, `#a2a0ad`, `#8b75ff`), `--tab=burn-in` opens the burn-in tab,
`-json -v --log-file` keeps stdout pure and writes the trace to both stderr and
the file, and `-q` suppresses a missing-job warning.

### P3 — Components: overlay, profile picker, file browser, progress, $EDITOR — **mostly done**

Landed in four commits (`ui: composite tab dialogs…`, `sysinfo: replace the
load-profile prompt…`, `sysinfo: browse for profile JSON files…`,
`tui: report job progress…`). No new modules except `bubbles/list`,
`bubbles/filepicker` (same module) and `github.com/charmbracelet/x/editor`.

What shipped:

| Slice | Result |
| --- | --- |
| Overlay compositor | `app.Tab` gained `Overlay(w, h) (string, bool)`; tabs keep rendering their normal view and hand the bare dialog to the shell, which centres it with `lipgloss.NewCompositor`. Help, apply review and prompts now float over the tab instead of replacing it; `Modal`/help set an explicit background so nothing bleeds through. |
| Profile picker | `ctrl+l` is a `bubbles/list` with a single-line theme-styled delegate, `/` filtering, and rebuilt items on every open. The list's own quit bindings are unbound so a stray key can never quit the program from a dialog. |
| Import browser | `ctrl+o` is a `bubbles/filepicker` confined to `*.json`, starting in the working directory, auto-sized in the dialog. `esc` closes the dialog; `h`/`backspace`/`←` still walk up. A validation failure keeps the dialog open with the reason inside it. |
| Native progress | `app.Tab.Progress() float64`; the root view maps it to `tea.View.ProgressBar` (OSC 9;4) so a long burn-in shows in the terminal itself. The in-panel ASCII bar is unchanged. |
| Job editing | burn-in `ctrl+e` opens the selected preset's file in `$EDITOR` via `x/editor` + `tea.ExecProcess`, then reloads the presets. The embedded preset has no file, so the binding explains `--job PATH`; editing is refused while a run is active. |

Deliberately **not** done:

- **`bubbles/table` for the run history.** Measured before implementing: a
  `table.Model` pads every cell, highlights row 0 as "selected" (needs styles
  neutralised), counts the header inside `WithHeight`, and expands columns to
  fill `WithWidth`. The two history blocks already use per-cell semantic colour
  (verdict → success/danger/warn) and hand-tuned widths that fit 80x24, so the
  table would add chrome, force a layout rewrite and change two goldens for no
  user-visible gain. The hand-rolled `ui.JoinColumns` tables stay.
- **`textarea` for job editing.** `$EDITOR` is the better tool and needs no
  in-TUI editor state machine.
- **First-run wizard.** There is nothing to configure on first launch.

Tests/goldens: new `profile_picker_118x37` golden (seeded with three profiles);
`help_*`, `apply_review_*` and `profile_prompt_*` regenerated for the overlay.
The import browser has no golden because it renders an absolute directory —
it is covered by a content/size test instead.

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

P4: `glamour` v2 for a `mem-station report --markdown` command that renders the
applied profile, SPD, IMC availability and recent history, reusing the existing
export schema instead of inventing a second one.

## 6. Open questions

1. Is breaking `-json` acceptable, or must the single-dash form stay forever?
   (Plan assumes it must stay, via the shim.)
2. Vendor dependencies (`go mod vendor`) now that the module path changes to a
   vanity domain, or rely on the proxy?

# Phase 2 handoff

- Commit: (this commit) `tui: phase 2 - timings workspace`
- Goal: make the Timings tab the real overclocking workspace: typed fields, an
  explicit detected/draft/applied state, validation with margins, an apply
  review modal, a five-column editor, and a live derived perf panel.

## Delivered

New:

- `internal/sysinfo/fields.go` - `FieldID`, `Field{ID, Label, Unit, Group,
  Kind, Options, Default, Desc, Step, Scale, Key, DetectLabel, Bounds, Format}`,
  the ten canonical fields (channels, ratio, freq, tCL, tRCD, tRP, tRAS, CR,
  tRFC, voltage), `Groups()`, `Fields()`, `LookupField()`, `Bounds` (with
  per-direction severity and a rendered `Label()`), `ValidationContext`, the
  bound functions (`primaryBounds`, `rasBounds`, `rfcBounds`), and the value
  parser/formatters.
- `internal/sysinfo/validate.go` - `Issue{FieldID, Severity, Message, Delta}`,
  `Validate`, `derivedIssues`, the margin helper, and `worstIssue`.

Changed:

- `internal/sysinfo/model.go` - three value maps (`detected`, `applied`,
  `draft`) keyed by `FieldID` instead of label-keyed structs; a single
  `textinput` for the field being edited; `baseline()` (applied -> detected ->
  placeholder); `Dirty()` draft-vs-baseline; `Snapshot()` now reports the
  applied profile (or detected values when none exists) with `Source`;
  `SetDetected` resolves detection keys by field ID, config key, or
  `memory.DetectDefaults` label, and only follows detection into a draft the
  user has not taken over; `ModalOpen()` now reflects the apply modal.
- `internal/sysinfo/update.go` - selection vs editing modes, Esc reverts the
  pre-edit value, Tab/Shift+Tab commits and moves, `ctrl+s` opens the review
  modal, `ctrl+r` resets the draft, the ratio lock rescales on commit only and
  clamps to the new minimums, `confirmApply` persists the active profile and
  reports "Profile only - not written to hardware".
- `internal/sysinfo/view.go` - five-column editor (Setting, Detected, Draft,
  Delta, Safety) built from computed column widths via `ui.JoinColumns`; the
  editor scrolls so the focused field is always visible; the apply review modal
  (diff, warnings, disclaimer) is centred in the body; the guidance panel gained
  the derived metrics (tRC/tRFC in ns), safety tallies, and the bandwidth delta
  against the applied profile with regressions highlighted.
- `internal/sysinfo/keys.go` - modal hints/help sections; ratio-lock and apply
  wording matches the new flow.
- `internal/sysinfo/bounds.go` - **deleted** (superseded by `fields.go`).
- `internal/app/update.go` - the modal/editing key path now forwards the applied
  snapshot to the run tabs after a confirmed apply.
- `internal/sysinfo/sysinfo_test.go` - rewritten for the ID-based API; adds
  validation tables (voltage, command rate, unknown values, derived tRAS), apply
  round-trip through the modal, apply-cancel, ratio-lock rescaling, detection
  following, draft reset, exact-size views, editor columns/windowing, and
  sub-view scroll retention.
- `internal/app/layout_test.go` - adds an end-to-end apply-review test through
  the shell (dirty marker, modal size, global-key gating, snapshot handoff).

## Contracts added/changed

- `sysinfo.Model.Snapshot()` now returns the **applied** values (Source
  `applied`, or `detected` when no profile exists) instead of the live draft.
  The run tabs therefore attribute a run to what was saved, not to typed input.
  `Draft()` was not added; the draft is internal.
- `sysinfo.Model.ModalOpen()` is real: `ctrl+s` opens the review, and the shell
  forwards every key to the tab while it is open (except `ctrl+c`).
- Field values stay strings; `FieldID` values equal the existing config keys, so
  profiles written before Phase 2 load unchanged.
- `Field.Scale` = ratio-lock rescale factor (1 rescales with the frequency
  ratio, 0 never); `Field.Step` = snap granularity for rescaled values.
  `Field.Format` is used for display only (voltage renders as `1.35 V`).

## Deviations from brief

- `Issue.Delta` and the Delta column use the margin to the nearest bound:
  negative under the safe minimum, positive above the safe maximum (and headroom
  when inside it). The Safety column disambiguates "over" from "headroom".
- tRFC-ns and tRC are **derived display metrics**, not validation issues: a
  nanosecond floor flags stock XMP profiles (16-18-18-38 at 3200 MT/s is 35 ns
  tRC) as unsafe. The tRAS 35 ns floor from Phase 0/1 was corrected for the same
  reason and replaced by the correct hard rule `tRAS >= tCL+tRCD`, which is also
  the only derived validation rule.
- The tRAS/tRFC/tCL upper bounds (18/63/1023 cycles) are retained from Phase 1
  as "above range" warnings, not dangers.
- Enter commits an edit without emitting a status message (Phase 1 announced
  "Value committed."); the footer already shows the mode.
- `q` inside the apply review cancels the review instead of quitting, so a
  stray key press cannot discard the draft silently or exit mid-review.

## Known risks / unverified areas

- Rendering is verified headlessly (exact cell counts, content presence) at
  78x21, 98x29 and 118x37 bodies; no human has looked at it in a real terminal.
- At 80x24 the editor shows ~8 of the ~19 table lines and scrolls with the
  focus; the guidance panel is 4 lines tall there. Nothing is clipped without an
  indicator, but the small-size experience is untested by eye.
- The "82% efficiency" sustained estimate is a heuristic carried over from Phase
  1 and is labelled as such; it is not measured.
- Validation remains heuristic (10 ns floor for tCL/tRCD/tRP, 160 ns floor for
  tRFC, 1.35-1.65 V). Only the tRAS >= tCL+tRCD rule is a hard timing rule.
- Ratio-lock rescaling scales the current drafts, not the detected baseline, and
  snaps to the field step; clamping only enforces the lower bound.
- No interactive run of the TUI, no `stress-ng`, no burn-in in this phase.

## Exact remaining work for Phase 3

Per `notes/impl-brief.md` §5 (Benchmark and Burn-in runs):

- Benchmark config (duration/buffer/workers) built as argv, run/cancel through
  `runner`, elapsed ticker, progress, follow/errors filter, parse on finish,
  history record with the applied snapshot, last-vs-previous delta, history
  table, JSON export, and error handling for a missing `stress-ng`.
- Burn-in presets (embedded `memory.job`), materialized timeout in the state
  dir, live filtered viewport with an error counter and banner, stop-on-error,
  distinct cancelled/failed/passed terminal states, and result persistence.
- Root integration: running markers already exist; verify event routing to an
  inactive tab and that the status line never overwrites an error banner.
- No Timings work is expected to block Phase 3. If a run should be attributed to
  the draft rather than the applied profile, change
  `sysinfo.Model.Snapshot()` and say so in the Phase 3 handoff.

## Commands intentionally not run

The TUI was never launched interactively, and no `stress-ng`, benchmark, or
burn-in run was executed. No hardware writes exist in the codebase.
`go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l .` were run
(this phase re-enabled verification, as in Phase 1).

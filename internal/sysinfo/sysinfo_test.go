package sysinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/store"
	"mem-station/internal/uitest"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// newTestModel builds a Timings tab with an isolated state directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	return New()
}

// detectedValues is a realistic detection result, keyed the way
// memory.DetectDefaults keys it.
func detectedValues() map[string]string {
	return map[string]string{
		"Channels":     "2",
		"DRAM Ratio":   "16",
		"DRAM Freq":    "3200",
		"CAS (tCL)":    "16",
		"tRCD":         "18",
		"tRP":          "18",
		"tRAS":         "38",
		"Command Rate": "1T",
		"tRFC":         "350",
		"DRAM Voltage": "1.35V",
	}
}

// detectedModel returns a tab with hardware detection already applied.
func detectedModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.SetDetected(detectedValues(), "Detected 2 channels at 3200 MT/s", nil)
	return m
}

// setDraft edits one field through the real editing lifecycle.
func setDraft(m *Model, id FieldID, value string) {
	m.focus = m.fieldIndex(id)
	m.startEditing()
	m.input.SetValue(value)
	m.draft[id] = value
	m.commitEdit()
}

// issuesFor returns the validation issues for one field.
func issuesFor(issues []Issue, id FieldID) []Issue {
	var out []Issue
	for _, iss := range issues {
		if iss.FieldID == id {
			out = append(out, iss)
		}
	}
	return out
}

// hasSeverityAtLeast reports whether any issue is at least as severe as min.
func hasSeverityAtLeast(issues []Issue, min Severity) bool {
	for _, iss := range issues {
		if iss.Severity >= min {
			return true
		}
	}
	return false
}

// TestParseNumber covers the value parser used for every numeric field.
func TestParseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1600", 1600, true},
		{"2T", 2, true},
		{"1.35V", 1.35, true},
		{"?", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseNumber(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseNumber(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestBoundsForFrequency checks the nanosecond-derived minimums and the fixed
// voltage window.
func TestBoundsForFrequency(t *testing.T) {
	ctx := NewValidationContext(map[FieldID]string{FieldFreq: "3200"})

	rfc, ok := LookupField(FieldRFC)
	if !ok {
		t.Fatal("tRFC field missing")
	}
	// tRFC >= 160 ns at 3200 MT/s = 3200*160/2000 = 256 cycles.
	if b := rfc.Bounds(ctx); b.Min != 256 {
		t.Errorf("tRFC min = %v, want 256", b.Min)
	}

	cl, _ := LookupField(FieldCL)
	// CAS minimum is max(5, 10 ns) = 16 cycles at 3200 MT/s.
	if b := cl.Bounds(ctx); b.Min != 16 {
		t.Errorf("tCL min = %v, want 16", b.Min)
	}

	v, _ := LookupField(FieldVoltage)
	if b := v.Bounds(ctx); b.Min != 1.35 || b.Max != 1.65 {
		t.Errorf("voltage bounds = %v-%v, want 1.35-1.65", b.Min, b.Max)
	}
}

// TestBoundsUnknownWithoutFrequency checks that no range is invented while the
// frequency is unknown.
func TestBoundsUnknownWithoutFrequency(t *testing.T) {
	ctx := NewValidationContext(map[FieldID]string{})
	cl, _ := LookupField(FieldCL)
	if b := cl.Bounds(ctx); b.Known() {
		t.Errorf("tCL bounds = %+v, want no range without a frequency", b)
	}
}

// TestValidateVoltage checks the 1.35-1.65 V window and its margins.
func TestValidateVoltage(t *testing.T) {
	cases := []struct {
		value string
		want  Severity
	}{
		{"1.35V", SeverityNone},
		{"1.50V", SeverityNone},
		{"1.20V", SeverityDanger},
		{"1.80V", SeverityDanger},
	}
	for _, c := range cases {
		issues := issuesFor(Validate(NewValidationContext(map[FieldID]string{FieldVoltage: c.value})), FieldVoltage)
		got := SeverityNone
		if len(issues) > 0 {
			got = issues[0].Severity
		}
		if got != c.want {
			t.Errorf("voltage %q: severity = %v, want %v", c.value, got, c.want)
		}
	}

	// The margin is reported in volts and is negative between the bound and
	// the measured value.
	issues := issuesFor(Validate(NewValidationContext(map[FieldID]string{FieldVoltage: "1.20V"})), FieldVoltage)
	if len(issues) != 1 {
		t.Fatalf("want one voltage issue, got %d", len(issues))
	}
	if got := issues[0].Delta; got > -0.14 || got < -0.16 {
		t.Errorf("voltage margin = %v, want about -0.15", got)
	}
}

// TestValidateCommandRate checks the enum rule for the command rate.
func TestValidateCommandRate(t *testing.T) {
	for _, value := range []string{"1T", "2T"} {
		if issues := issuesFor(Validate(NewValidationContext(map[FieldID]string{FieldCR: value})), FieldCR); len(issues) != 0 {
			t.Errorf("CR %q produced %d issues, want 0", value, len(issues))
		}
	}
	issues := issuesFor(Validate(NewValidationContext(map[FieldID]string{FieldCR: "3T"})), FieldCR)
	if len(issues) != 1 || issues[0].Severity != SeverityDanger {
		t.Errorf("CR 3T issues = %+v, want one danger", issues)
	}
}

// TestValidateUnknownIsInfo checks that unknown values report "waiting for
// detection" instead of a fake range.
func TestValidateUnknownIsInfo(t *testing.T) {
	issues := Validate(NewValidationContext(map[FieldID]string{}))
	if len(issues) == 0 {
		t.Fatal("empty draft produced no issues")
	}
	for _, iss := range issues {
		if iss.Severity != SeverityInfo {
			t.Errorf("issue for %s = %+v, want info", iss.FieldID, iss)
		}
		if !strings.Contains(iss.Message, "waiting for detection") {
			t.Errorf("issue message = %q, want a detection note", iss.Message)
		}
	}
}

// TestValidateDerivedRules checks the cross-field timing rules.
func TestValidateDerivedRules(t *testing.T) {
	// tRAS below tCL+tRCD is a danger.
	values := map[FieldID]string{
		FieldFreq: "3200", FieldCL: "16", FieldRCD: "18", FieldRAS: "30", FieldRP: "18",
	}
	issues := issuesFor(Validate(NewValidationContext(values)), FieldRAS)
	var found bool
	for _, iss := range issues {
		if iss.Severity == SeverityDanger && strings.Contains(iss.Message, "tCL+tRCD") {
			found = true
			if iss.Delta >= 0 {
				t.Errorf("tRAS delta = %v, want negative", iss.Delta)
			}
		}
	}
	if !found {
		t.Errorf("tRAS 30 with tCL 16 / tRCD 18 did not raise a derived danger: %+v", issues)
	}

	// A stock XMP profile (16-18-18-38 at 3200 MT/s) must validate clean: a
	// nanosecond floor for tRAS or tRC would flag real hardware as unsafe.
	values[FieldRAS] = "38"
	if issues := Validate(NewValidationContext(values)); hasSeverityAtLeast(issues, SeverityWarn) {
		t.Errorf("stock XMP timings produced issues: %+v", issues)
	}

	// tRFC below 160 ns is unsafe.
	issues = issuesFor(Validate(NewValidationContext(map[FieldID]string{
		FieldFreq: "3200", FieldRFC: "100",
	})), FieldRFC)
	if len(issues) == 0 || issues[0].Severity != SeverityDanger {
		t.Errorf("tRFC 100 cycles at 3200 MT/s issues = %+v, want danger", issues)
	}
}

// TestEditApplyRoundTrip checks draft editing, the dirty flag, the review
// modal, and that apply persists the active profile without touching hardware.
func TestEditApplyRoundTrip(t *testing.T) {
	m := newTestModel(t)

	if m.Dirty() {
		t.Fatal("a fresh model must not be dirty")
	}

	// Edit the first field: select it, start editing, type a digit, commit.
	m.Update(uitest.Code(tea.KeyEnter))
	m.Update(uitest.Key("2"))
	m.Update(uitest.Code(tea.KeyEnter))

	if !m.Dirty() {
		t.Fatal("editing a field must mark the draft dirty")
	}

	// ctrl+s opens the review; the modal owns the keys until confirmed.
	if cmd := m.Update(uitest.Ctrl('s')); cmd != nil {
		t.Error("opening the apply review must not emit a status message")
	}
	if !m.ModalOpen() {
		t.Fatal("ctrl+s did not open the apply review")
	}
	dialog, ok := m.Overlay(78, 21)
	if !ok {
		t.Fatal("ctrl+s did not expose an overlay dialog")
	}
	if !strings.Contains(dialog, "Profile only - not written to hardware") {
		t.Errorf("apply review is missing the hardware disclaimer:\n%s", dialog)
	}
	if got := lipgloss.Height(dialog); got > 21 {
		t.Errorf("apply review height = %d, want at most 21", got)
	}
	if got := lipgloss.Width(dialog); got > 78 {
		t.Errorf("apply review width = %d, want at most 78", got)
	}

	if cmd := m.Update(uitest.Code(tea.KeyEnter)); cmd == nil {
		t.Fatal("confirming the apply must return a status message")
	}
	if m.ModalOpen() || m.Dirty() {
		t.Error("apply must close the modal and clear the dirty flag")
	}

	p, ok, err := store.LoadActiveProfile()
	if err != nil || !ok {
		t.Fatalf("active profile missing after apply: ok=%v err=%v", ok, err)
	}
	if got := p.Values[config.KeyChannels]; got == "" {
		t.Errorf("applied profile has no channels value: %+v", p.Values)
	}
}

// TestApplyNotDirty checks that applying an unchanged draft is a no-op.
func TestApplyNotDirty(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Update(uitest.Ctrl('s'))
	if cmd == nil {
		t.Fatal("apply without changes must report a status message")
	}
	if m.ModalOpen() {
		t.Error("apply without changes must not open the review")
	}
}

// TestApplyCancelKeepsDraft checks that esc closes the review and saves nothing.
func TestApplyCancelKeepsDraft(t *testing.T) {
	m := newTestModel(t)
	setDraft(m, FieldVoltage, "1.50")

	m.Update(uitest.Ctrl('s'))
	if !m.ModalOpen() {
		t.Fatal("ctrl+s did not open the apply review")
	}
	if cmd := m.Update(uitest.Code(tea.KeyEsc)); cmd == nil {
		t.Error("cancelling the apply must report a status message")
	}
	if m.ModalOpen() {
		t.Error("esc did not close the apply review")
	}
	if !m.Dirty() {
		t.Error("cancelling the apply must keep the draft dirty")
	}
	if _, ok, _ := store.LoadActiveProfile(); ok {
		t.Error("cancelling the apply must not write a profile")
	}
}

// TestCancelEditRestoresValue checks esc while editing only reverts the edit.
func TestCancelEditRestoresValue(t *testing.T) {
	m := newTestModel(t)
	before := m.draft[FieldChannels]

	m.Update(uitest.Code(tea.KeyEnter))
	m.Update(uitest.Key("9"))
	m.Update(uitest.Code(tea.KeyEsc))

	if got := m.draft[FieldChannels]; got != before {
		t.Errorf("esc while editing: value = %q, want %q", got, before)
	}
	if m.Dirty() {
		t.Error("cancelled edit must not leave the draft dirty")
	}
}

// TestSnapshotUsesCanonicalKeys checks the snapshot handed to the run tabs.
func TestSnapshotUsesCanonicalKeys(t *testing.T) {
	m := newTestModel(t)
	snap := m.Snapshot()
	if snap.Source != "detected" {
		t.Errorf("snapshot source = %q, want detected without a profile", snap.Source)
	}
	for _, f := range Fields() {
		if _, ok := snap.Values[f.Key]; !ok {
			t.Errorf("snapshot missing key %q", f.Key)
		}
	}
}

// TestSnapshotTracksAppliedProfile checks that the run tabs see the applied
// profile, not the editable draft.
func TestSnapshotTracksAppliedProfile(t *testing.T) {
	m := detectedModel(t)
	setDraft(m, FieldFreq, "2400")
	m.Update(uitest.Ctrl('s'))
	m.Update(uitest.Code(tea.KeyEnter))

	snap := m.Snapshot()
	if snap.Source != "applied" {
		t.Errorf("snapshot source = %q, want applied", snap.Source)
	}
	if got := snap.Values[config.KeyFreq]; got != "2400" {
		t.Errorf("snapshot freq = %q, want 2400", got)
	}
	if got := m.draft[FieldFreq]; got != "2400" {
		t.Errorf("draft freq = %q, want 2400", got)
	}
}

// TestDetectionFollowsUntouchedDraft checks that detection fills the draft
// until the user or an applied profile owns the value.
func TestDetectionFollowsUntouchedDraft(t *testing.T) {
	m := newTestModel(t)
	if got := m.draft[FieldFreq]; got != "1600" {
		t.Fatalf("placeholder freq = %q, want 1600", got)
	}
	m.SetDetected(detectedValues(), "ok", nil)
	if got := m.draft[FieldFreq]; got != "3200" {
		t.Errorf("draft freq after detection = %q, want 3200", got)
	}
	if got := m.detected[FieldCL]; got != "16" {
		t.Errorf("detected tCL = %q, want 16", got)
	}

	// An applied profile wins over later detection.
	setDraft(m, FieldFreq, "2400")
	m.Update(uitest.Ctrl('s'))
	m.Update(uitest.Code(tea.KeyEnter))
	m.SetDetected(detectedValues(), "ok", nil)
	if got := m.draft[FieldFreq]; got != "2400" {
		t.Errorf("applied freq overwritten by detection: %q", got)
	}
}

// TestRatioLockRescalesOnCommit checks that committing a frequency edit scales
// the primary timings, and that the lock disables it.
func TestRatioLockRescalesOnCommit(t *testing.T) {
	m := detectedModel(t)
	if !m.lockRatios {
		t.Fatal("ratio lock must default to on")
	}
	setDraft(m, FieldFreq, "3600")

	// 16 cycles * 3600/3200 = 18, exactly the new 10 ns floor.
	if got := m.draft[FieldCL]; got != "18" {
		t.Errorf("tCL after rescale = %q, want 18", got)
	}

	m.lockRatios = false
	setDraft(m, FieldVoltage, "1.40")
	setDraft(m, FieldFreq, "3200")
	if got := m.draft[FieldCL]; got != "18" {
		t.Errorf("tCL changed with the ratio lock off: %q", got)
	}
}

// TestResetDraft checks ctrl+r restores the applied/detected values.
func TestResetDraft(t *testing.T) {
	m := detectedModel(t)
	setDraft(m, FieldCL, "20")
	if !m.Dirty() {
		t.Fatal("draft should be dirty after an edit")
	}
	m.Update(uitest.Ctrl('r'))
	if m.Dirty() {
		t.Error("ctrl+r must clear the dirty flag")
	}
	if got := m.draft[FieldCL]; got != "16" {
		t.Errorf("tCL after reset = %q, want the detected 16", got)
	}
}

// TestViewIsExactSize checks the editor and every sub-view fill w x h exactly.
func TestViewIsExactSize(t *testing.T) {
	m := detectedModel(t)
	m.SetDetected(detectedValues(), "ok", &memory.SPDInfo{
		PartNumber:   "F4-3200C16-16GVK",
		SizeMB:       "16384",
		ModuleSpeed:  "3200 MT/s",
		TimingString: "16-18-18-38",
	})

	sizes := []struct{ w, h int }{{78, 21}, {98, 29}, {118, 37}}
	for _, size := range sizes {
		for sub := 0; sub < 3; sub++ {
			if sub > 0 {
				m.cycleView(1)
			}
			out := m.View(size.w, size.h)
			if got := lipgloss.Height(out); got != size.h {
				t.Errorf("%s %dx%d: height = %d", subView(sub), size.w, size.h, got)
			}
			if got := lipgloss.Width(out); got != size.w {
				t.Errorf("%s %dx%d: width = %d", subView(sub), size.w, size.h, got)
			}
		}
		m.subView = subTimings
	}
}

// TestEditorColumns checks the aligned column headers and the values they hold.
func TestEditorColumns(t *testing.T) {
	m := detectedModel(t)
	out := m.View(78, 21)
	for _, want := range []string{"Setting", "Detected", "Draft", "Margin", "Safety"} {
		if !strings.Contains(out, want) {
			t.Errorf("editor is missing the %q column header:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "16") {
		t.Errorf("editor does not show the detected tCL:\n%s", out)
	}
}

// TestEditorWindowKeepsFocusVisible checks the editor scrolls so the focused
// field is always rendered, even in a short body.
func TestEditorWindowKeepsFocusVisible(t *testing.T) {
	m := detectedModel(t)
	m.focus = m.fieldIndex(FieldVoltage)
	out := m.View(78, 21)
	if !strings.Contains(out, "DRAM Voltage") {
		t.Errorf("focused field is not visible in a 78x21 body:\n%s", out)
	}
}

// TestSubViewScrollRetained checks each sub-view keeps its scroll offset across
// renders.
func TestSubViewScrollRetained(t *testing.T) {
	m := newTestModel(t)
	m.SetDetected(detectedValues(), "ok", &memory.SPDInfo{
		PartNumber:     "F4-3200C16-16GVK",
		SizeMB:         "16384",
		ModuleSpeed:    "3200 MT/s",
		TimingString:   "16-18-18-38",
		TimingsBySpeed: []string{"DDR4-3200 16-18-18-38", "DDR4-2133 15-15-15-36"},
	})
	m.Update(uitest.Key("]"))
	m.View(78, 21)
	m.Update(uitest.Code(tea.KeyPgDown))

	scrolled := m.spdVP.YOffset()
	if scrolled == 0 {
		t.Fatal("pgdown did not scroll the SPD view")
	}
	m.View(78, 21)
	if got := m.spdVP.YOffset(); got != scrolled {
		t.Errorf("scroll offset changed on re-render: %d, want %d", got, scrolled)
	}
}

/* --------------- Profiles and JSON import/export --------------- */

// promptModel runs a profile key and returns the tab with its prompt modal
// open, typing value into the prompt the way a user would.
func promptModel(t *testing.T, m *Model, key tea.KeyPressMsg, value string) {
	t.Helper()
	m.Update(key)
	if !m.ModalOpen() || !m.modal.prompt() {
		t.Fatalf("%v did not open a prompt modal", key)
	}
	m.prompt.SetValue("") // prompts may be prefilled; the test types its own value
	for _, r := range value {
		m.Update(uitest.Key(string(r)))
	}
}

// pickProfile opens the load picker, highlights name, and confirms it.
func pickProfile(t *testing.T, m *Model, name string) {
	t.Helper()
	m.Update(uitest.Ctrl('l'))
	if !m.ModalOpen() || m.modal != modalLoadProfile {
		t.Fatalf("ctrl+l did not open the profile picker (modal %v)", m.modal)
	}
	index := -1
	for i, item := range m.picker.Items() {
		if profile, ok := item.(profileItem); ok && profile.name == name {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("profile %q is not in the picker: %v", name, m.picker.Items())
	}
	m.picker.Select(index)
	m.Update(uitest.Code(tea.KeyEnter))
}

// openImport opens the JSON file browser at path's directory and highlights it.
func openImport(t *testing.T, m *Model, path string) {
	t.Helper()
	m.Update(uitest.Ctrl('o'))
	if !m.ModalOpen() || m.modal != modalImportProfile {
		t.Fatalf("ctrl+o did not open the import browser (modal %v)", m.modal)
	}
	m.importPicker.CurrentDirectory = filepath.Dir(path)
	if cmd := m.importPicker.Init(); cmd != nil {
		m.Update(cmd())
	}
	highlightImport(t, m, path)
}

// highlightImport moves the browser selection onto path.
func highlightImport(t *testing.T, m *Model, path string) {
	t.Helper()
	for i := 0; i < 64 && m.importPicker.HighlightedPath() != path; i++ {
		m.Update(uitest.Code(tea.KeyDown))
	}
	if got := m.importPicker.HighlightedPath(); got != path {
		t.Fatalf("could not highlight %s in the import browser (at %s)", path, got)
	}
}

// TestSaveNamedProfileFromDraft saves the draft under a name without touching
// the active profile.
func TestSaveNamedProfileFromDraft(t *testing.T) {
	m := detectedModel(t)
	setDraft(m, FieldChannels, "4")
	promptModel(t, m, uitest.Ctrl('p'), "daily")
	if cmd := m.Update(uitest.Code(tea.KeyEnter)); cmd == nil {
		t.Fatal("confirming the save produced no status message")
	}
	if m.ModalOpen() {
		t.Error("the modal stayed open after a successful save")
	}
	saved, ok, err := store.LoadNamedProfile("daily")
	if err != nil || !ok {
		t.Fatalf("LoadNamedProfile(daily) = %v, %v", ok, err)
	}
	if got := saved.Values[config.KeyChannels]; got != "4" {
		t.Errorf("saved channels = %q, want the draft value 4", got)
	}
	if !m.Dirty() {
		t.Error("saving a named profile applied the draft to the active profile")
	}
}

// TestLoadNamedProfileIntoDraft loads a saved profile into the draft only.
func TestLoadNamedProfileIntoDraft(t *testing.T) {
	m := detectedModel(t)
	if err := store.SaveNamedProfile(config.Profile{
		Name:   "undervolt",
		Values: map[string]string{config.KeyFreq: "2933", config.KeyVoltage: "1.20"},
	}); err != nil {
		t.Fatalf("SaveNamedProfile: %v", err)
	}

	pickProfile(t, m, "undervolt")

	if got := m.draft[FieldFreq]; got != "2933" {
		t.Errorf("draft frequency = %q, want 2933", got)
	}
	if !m.Dirty() {
		t.Error("a loaded profile did not mark the draft dirty")
	}
	if _, ok := m.applied[FieldFreq]; ok {
		t.Error("loading a named profile changed the applied profile")
	}
}

// TestImportProfileValidation keeps the modal open and the draft untouched when
// the file is invalid, then loads a valid profile.
func TestImportProfileValidation(t *testing.T) {
	m := detectedModel(t)
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"name":"bad","values":{"not_a_key":"1"}}`), 0o600); err != nil {
		t.Fatalf("write bad profile: %v", err)
	}
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"name":"good","values":{"tCL":"18","tRCD":"20"}}`), 0o600); err != nil {
		t.Fatalf("write good profile: %v", err)
	}
	openImport(t, m, bad)
	m.Update(uitest.Code(tea.KeyEnter))
	if !m.ModalOpen() {
		t.Fatal("an invalid import closed the modal")
	}
	if m.promptErr == "" {
		t.Error("an invalid import did not explain the failure")
	}
	if m.Dirty() {
		t.Error("an invalid import changed the draft")
	}

	highlightImport(t, m, good)
	m.Update(uitest.Code(tea.KeyEnter))
	if m.ModalOpen() {
		t.Error("a valid import did not close the modal")
	}
	if got := m.draft[FieldCL]; got != "18" {
		t.Errorf("imported tCL = %q, want 18", got)
	}
	if got := m.draft[FieldRCD]; got != "20" {
		t.Errorf("imported tRCD = %q, want 20", got)
	}
}

// TestExportProfileToPath writes the active profile to a user-chosen path and
// verifies it round-trips through the validated importer.
func TestExportProfileToPath(t *testing.T) {
	m := detectedModel(t)
	path := filepath.Join(t.TempDir(), "nested", "profile.json")

	promptModel(t, m, uitest.Ctrl('e'), path)
	if cmd := m.Update(uitest.Code(tea.KeyEnter)); cmd == nil {
		t.Fatal("exporting produced no status message")
	}
	if m.ModalOpen() {
		t.Error("the export modal stayed open after a successful write")
	}
	profile, err := store.ImportProfile(path)
	if err != nil {
		t.Fatalf("ImportProfile(%s): %v", path, err)
	}
	if got := profile.Values[config.KeyFreq]; got != "3200" {
		t.Errorf("exported frequency = %q, want the detected 3200", got)
	}
	if profile.Source != "detected" {
		t.Errorf("exported source = %q, want detected", profile.Source)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("export mode = %o, want 600", perm)
	}
}

// TestProfilePromptEmptyValue checks an empty prompt is rejected with a reason
// instead of silently closing.
func TestProfilePromptEmptyValue(t *testing.T) {
	m := detectedModel(t)
	m.Update(uitest.Ctrl('p'))
	m.Update(uitest.Code(tea.KeyEnter))
	if !m.ModalOpen() {
		t.Fatal("an empty prompt closed the modal")
	}
	if !strings.Contains(m.promptErr, "required") {
		t.Errorf("prompt error = %q, want a required-value message", m.promptErr)
	}
	// esc always closes and leaves the draft alone.
	m.Update(uitest.Code(tea.KeyEsc))
	if m.ModalOpen() {
		t.Error("esc did not close the prompt")
	}
}

/* --------------- Field navigation and input masking --------------- */

// TestArrowKeysAdjustNumericFields checks the selected value moves by the
// field's declared step, without entering the text input.
func TestArrowKeysAdjustNumericFields(t *testing.T) {
	m := newTestModel(t)

	m.focus = m.fieldIndex(FieldCL)
	m.draft[FieldCL] = "16"
	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldCL]; got != "17" {
		t.Errorf("tCL after → = %q, want 17", got)
	}
	m.Update(uitest.Code(tea.KeyLeft))
	if got := m.draft[FieldCL]; got != "16" {
		t.Errorf("tCL after ← = %q, want 16", got)
	}

	m.focus = m.fieldIndex(FieldFreq)
	m.draft[FieldFreq] = "3200"
	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldFreq]; got != "3300" {
		t.Errorf("freq after → = %q, want the 100 MT/s step", got)
	}

	m.focus = m.fieldIndex(FieldVoltage)
	m.draft[FieldVoltage] = "1.35"
	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldVoltage]; got != "1.36" {
		t.Errorf("voltage after → = %q, want 1.36", got)
	}
}

// TestArrowKeysCyclePresetValues checks enum fields use their allowed values
// and wrap around, and Enter does not open an editor for them.
func TestArrowKeysCyclePresetValues(t *testing.T) {
	m := newTestModel(t)
	m.focus = m.fieldIndex(FieldCR)
	m.draft[FieldCR] = "1T"

	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldCR]; got != "2T" {
		t.Errorf("command rate after → = %q, want 2T", got)
	}
	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldCR]; got != "1T" {
		t.Errorf("command rate did not wrap: %q", got)
	}
	m.Update(uitest.Code(tea.KeyLeft))
	if got := m.draft[FieldCR]; got != "2T" {
		t.Errorf("command rate after ← = %q, want 2T", got)
	}
	if cmd := m.Update(uitest.Code(tea.KeyEnter)); cmd == nil {
		t.Error("Enter on a preset-only field should explain the ←/→ keys")
	}
	if m.editing {
		t.Error("Enter opened a text editor for a preset-only field")
	}
}

// TestArrowKeysUseSPDFrequencies checks the SPD speed list becomes the
// frequency picker while still allowing a custom value via Enter.
func TestArrowKeysUseSPDFrequencies(t *testing.T) {
	m := newTestModel(t)
	m.SetDetected(detectedValues(), "ok", &memory.SPDInfo{
		ModuleSpeed:    "3200 MT/s",
		TimingsBySpeed: []string{"DDR4-2133 15-15-15-36", "DDR4-3200 16-18-18-38", "DDR4-3600 18-20-20-40"},
	})
	m.focus = m.fieldIndex(FieldFreq)
	m.draft[FieldFreq] = "3200"

	m.Update(uitest.Code(tea.KeyRight))
	if got := m.draft[FieldFreq]; got != "3600" {
		t.Errorf("freq after → = %q, want the next SPD speed 3600", got)
	}
	m.Update(uitest.Code(tea.KeyLeft))
	m.Update(uitest.Code(tea.KeyLeft))
	if got := m.draft[FieldFreq]; got != "2133" {
		t.Errorf("freq after ← ← = %q, want the previous SPD speed 2133", got)
	}
	if !m.startEditing() {
		t.Error("SPD frequency presets must not prevent a custom frequency")
	}
}

// TestNumericInputMasking checks integer and float editors reject characters
// they cannot store, including pasted text.
func TestNumericInputMasking(t *testing.T) {
	m := newTestModel(t)
	m.focus = m.fieldIndex(FieldCL)
	m.startEditing()
	m.Update(uitest.Key("1a2."))
	if got := m.input.Value(); got != "12" {
		t.Errorf("integer input = %q, want digits only", got)
	}
	m.cancelEdit()

	m.focus = m.fieldIndex(FieldVoltage)
	m.startEditing()
	m.Update(uitest.Key("1.3x5"))
	if got := m.input.Value(); got != "1.35" {
		t.Errorf("float input = %q, want one decimal point", got)
	}
	m.Update(uitest.Key("."))
	if got := m.input.Value(); got != "1.35" {
		t.Errorf("float input accepted a second decimal point: %q", got)
	}
}

// TestArrowNavigationStillMovesFocus keeps the existing Up/Down behaviour
// explicit now that Left/Right are also handled.
func TestArrowNavigationStillMovesFocus(t *testing.T) {
	m := newTestModel(t)
	m.focus = 0
	m.Update(uitest.Code(tea.KeyDown))
	if m.focus != 1 {
		t.Errorf("↓ moved focus to %d, want 1", m.focus)
	}
	m.Update(uitest.Code(tea.KeyUp))
	if m.focus != 0 {
		t.Errorf("↑ moved focus to %d, want 0", m.focus)
	}
}

// TestProfilePickerClosesWithoutQuitting checks q and esc close the picker and
// that the list's own quit binding can never take the program down.
func TestProfilePickerClosesWithoutQuitting(t *testing.T) {
	m := detectedModel(t)
	if err := store.SaveNamedProfile(config.Profile{Name: "stock"}); err != nil {
		t.Fatalf("SaveNamedProfile: %v", err)
	}

	for _, key := range []tea.KeyPressMsg{uitest.Key("q"), uitest.Code(tea.KeyEsc)} {
		m.Update(uitest.Ctrl('l'))
		if !m.ModalOpen() {
			t.Fatalf("%v: ctrl+l did not open the picker", key)
		}
		cmd := m.Update(key)
		if m.ModalOpen() {
			t.Errorf("%v did not close the picker", key)
		}
		if cmd == nil {
			continue
		}
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Errorf("%v from the picker asked the program to quit", key)
		}
	}
}

// TestProfilePickerEmptyState checks opening the picker with no saved profiles
// is not an error and confirming loads nothing.
func TestProfilePickerEmptyState(t *testing.T) {
	m := detectedModel(t)
	m.Update(uitest.Ctrl('l'))
	if !m.ModalOpen() {
		t.Fatal("ctrl+l did not open the picker")
	}
	if got := len(m.picker.Items()); got != 0 {
		t.Fatalf("picker items = %d, want none", got)
	}
	m.Update(uitest.Code(tea.KeyEnter))
	if m.Dirty() {
		t.Error("confirming an empty picker changed the draft")
	}
}

// TestImportDialogLayout checks the file browser renders inside the dialog
// frame, names the directory and files, and closes on esc without importing.
func TestImportDialogLayout(t *testing.T) {
	m := detectedModel(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"name":"x"}`), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	m.Update(uitest.Ctrl('o'))
	m.importPicker.CurrentDirectory = dir
	m.Update(m.importPicker.Init()())

	dialog, ok := m.Overlay(78, 21)
	if !ok {
		t.Fatal("ctrl+o did not expose an overlay dialog")
	}
	for _, want := range []string{"Import profile JSON", "profile.json", "esc cancel"} {
		if !strings.Contains(dialog, want) {
			t.Errorf("import dialog missing %q:\n%s", want, dialog)
		}
	}
	if got := lipgloss.Width(dialog); got > 78 {
		t.Errorf("import dialog width = %d, want at most 78", got)
	}
	if got := lipgloss.Height(dialog); got > 21 {
		t.Errorf("import dialog height = %d, want at most 21", got)
	}

	m.Update(uitest.Code(tea.KeyEsc))
	if m.ModalOpen() {
		t.Error("esc did not close the import browser")
	}
	if m.Dirty() {
		t.Error("closing the import browser changed the draft")
	}
}

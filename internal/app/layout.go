package app

// Terminal layout constants. The shell is a fixed 3-row frame (header, tab bar,
// footer) around a body that fills the remaining rows exactly.
const (
	minWidth  = 80
	minHeight = 24

	headerH = 1
	tabBarH = 1
	footerH = 1

	// twoColumnMin is the body width at which tabs may use two columns.
	twoColumnMin = 110
)

// frame is the current layout geometry, all in terminal cells.
type frame struct {
	width    int // full terminal width
	height   int // full terminal height
	contentW int // width inside the horizontal padding
	bodyH    int // exact height handed to the active tab
}

// size returns the current terminal size with a sane fallback before the first
// WindowSizeMsg.
func (m *Model) size() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	return w, h
}

// frame computes the layout geometry.
func (m *Model) frame() frame {
	w, h := m.size()
	contentW := w - 2 // one column of padding on each side
	if contentW < 1 {
		contentW = 1
	}
	bodyH := h - headerH - tabBarH - footerH
	if bodyH < 1 {
		bodyH = 1
	}
	return frame{width: w, height: h, contentW: contentW, bodyH: bodyH}
}

// tooSmall reports whether the terminal cannot host the fixed shell.
func (m *Model) tooSmall() bool {
	w, h := m.size()
	return w < minWidth || h < minHeight
}

// twoColumn reports whether the body is wide enough for side-by-side panels.
func (f frame) twoColumn() bool { return f.contentW >= twoColumnMin }

package rendersystem

import (
	"crossterm/internal/engine"
	"crossterm/internal/puzzle"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

const cellW = 4 // horizontal chars per cell (border + content)
const cellH = 2 // vertical chars per cell (border + content rows)

// Cliptic-inspired colors
var (
	ColorBg        = tcell.ColorDefault
	ColorBorder    = tcell.ColorTeal
	ColorBlackSq   = tcell.ColorTeal
	ColorText      = tcell.ColorLightGreen
	ColorHighlight = tcell.ColorWhite
	ColorHlText    = tcell.ColorBlack

	// Across highlight = yellow family
	ColorAcrossHl   = tcell.NewRGBColor(120, 120, 0)
	ColorAcrossText = tcell.ColorWhite

	// Down highlight = purple/magenta family
	ColorDownHl   = tcell.NewRGBColor(80, 40, 120)
	ColorDownText = tcell.ColorWhite

	ColorHeaderBg = tcell.ColorSteelBlue
	ColorHeaderFg = tcell.ColorBlack

	ColorClueBoxBg  = tcell.ColorDefault
	ColorClueBorder = tcell.ColorTeal
	ColorClueText   = tcell.ColorLightGreen

	ColorAcrossClueDir = tcell.NewRGBColor(220, 220, 0)
	ColorDownClueDir   = tcell.NewRGBColor(180, 100, 220)

	ColorStatusBg = tcell.ColorSteelBlue
	ColorStatusFg = tcell.ColorBlack
)

type RenderSystem struct {
	Screen   tcell.Screen
	EventBus *engine.EventBus
	State    *engine.GameState
}

func NewRenderSystem(screen tcell.Screen, eb *engine.EventBus, state *engine.GameState) *RenderSystem {
	return &RenderSystem{
		Screen:   screen,
		EventBus: eb,
		State:    state,
	}
}

func (s *RenderSystem) Run() {
	updateSub := s.EventBus.Subscribe(engine.EventStateUpdate)

	s.Paint()

	// Add a ticker to force UI updates for the timer every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-updateSub:
			s.Paint()
		case <-ticker.C:
			s.Paint()
		}
	}
}

func (s *RenderSystem) Paint() {
	s.Screen.Clear()

	if s.State.Puzzle == nil {
		w, h := s.Screen.Size()
		msg := " Waiting for Host to sync puzzle... "
		style := tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite)
		drawString(s.Screen, (w-len(msg))/2, h/2, msg, style)
		s.Screen.Show()
		return
	}

	drawHeader(s.Screen, s.State)
	drawGrid(s.Screen, s.State)
	clueLines := 0
	if s.State.ShowAllClues {
		clueLines = drawAllCluesBox(s.Screen, s.State)
	} else {
		clueLines = drawClueBox(s.Screen, s.State)
	}

	counterLines := 0
	if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
		counterLines = drawCounter(s.Screen, s.State, clueLines)
	}

	drawStatus(s.Screen, s.State, clueLines+counterLines)

	s.Screen.Show()
}

func drawHeader(screen tcell.Screen, state *engine.GameState) {
	w, _ := screen.Size()

	style := tcell.StyleDefault.Background(ColorHeaderBg).Foreground(ColorHeaderFg)
	var title string
	if state.Puzzle != nil {
		title = fmt.Sprintf(" cryptic: %s - %s ", state.Puzzle.Title, state.Puzzle.Author)
	} else {
		title = " cryptic: syncing... "
	}

	progressWidth := 0
	if !state.IsDuel && !strings.HasSuffix(state.Mode, "standard") && state.Puzzle != nil && state.Puzzle.Grid != nil {
		correct, total := 0, 0
		grid := state.Puzzle.Grid
		for y := 0; y < grid.Height; y++ {
			for x := 0; x < grid.Width; x++ {
				if !grid.Cells[y][x].IsBlack {
					total++
					if grid.Cells[y][x].Value == grid.Cells[y][x].Solution {
						correct++
					}
				}
			}
		}
		if total > 0 {
			progressWidth = int(float64(w) * float64(correct) / float64(total))
		}
	}

	progressColor := tcell.ColorCadetBlue

	drawProgStr := func(startX int, s string, st tcell.Style) {
		x := startX
		for _, r := range s {
			curSt := st
			if x < progressWidth && !state.IsFinished {
				curSt = st.Background(progressColor)
			}
			screen.SetContent(x, 0, r, nil, curSt)
			x += runewidth.RuneWidth(r)
		}
	}

	for x := 0; x < w; x++ {
		bg := ColorHeaderBg
		if x < progressWidth && !state.IsFinished {
			bg = progressColor
		}
		screen.SetContent(x, 0, ' ', nil, style.Background(bg))
	}

	drawProgStr(0, title, style)

	if !strings.HasPrefix(state.Mode, "not_timed") {
		var elapsed time.Duration
		if state.IsFinished {
			elapsed = state.FinalTime
		} else {
			elapsed = time.Since(state.StartTime)
			if state.PenaltyTime > 0 {
				elapsed += state.PenaltyTime
			}
		}

		timer := fmt.Sprintf(" %02d:%02d:%02d ", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
		if state.PenaltyTime > 0 {
			timer = fmt.Sprintf(" %s (+%ds penalty) ", timer, int(state.PenaltyTime.Seconds()))
		}

		if state.IsFinished {
			timer = fmt.Sprintf(" [ %s ] %s ", state.Status, timer)
			switch state.Status {
			case "WON (Perfect)", "WON":
				style = style.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack)
			case "RESIGNED":
				style = style.Background(tcell.ColorRed).Foreground(tcell.ColorWhite)
			case "DRAW":
				style = style.Background(tcell.ColorDarkGray).Foreground(tcell.ColorWhite)
			default:
				style = style.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack)
			}
		}

		drawProgStr(w-runewidth.StringWidth(timer), timer, style)
	}
}

func getHighlightBounds(state *engine.GameState) (int, int, int, int) {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return 0, 0, 0, 0
	}
	grid := state.Puzzle.Grid
	cx, cy := state.Cursor.X, state.Cursor.Y
	dir := state.Cursor.Direction

	hlStartX, hlStartY, hlEndX, hlEndY := cx, cy, cx, cy

	if dir == puzzle.DirAcross {
		for hlStartX > 0 && !grid.GetCell(hlStartX-1, cy).IsBlack {
			hlStartX--
		}
		for hlEndX < grid.Width-1 && !grid.GetCell(hlEndX+1, cy).IsBlack {
			hlEndX++
		}
	} else {
		for hlStartY > 0 && !grid.GetCell(cx, hlStartY-1).IsBlack {
			hlStartY--
		}
		for hlEndY < grid.Height-1 && !grid.GetCell(cx, hlEndY+1).IsBlack {
			hlEndY++
		}
	}
	return hlStartX, hlStartY, hlEndX, hlEndY
}

func gridPixelWidth(grid *puzzle.Grid) int {
	return grid.Width*cellW + 1
}

func drawGrid(screen tcell.Screen, state *engine.GameState) {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return
	}
	grid := state.Puzzle.Grid

	startX, startY := 1, 2
	styleBorder := tcell.StyleDefault.Foreground(ColorBorder).Background(ColorBg)

	// Draw borders
	for y := 0; y <= grid.Height; y++ {
		for x := 0; x <= grid.Width; x++ {
			drY := startY + y*cellH
			drX := startX + x*cellW

			// Determine intersection char
			char := '┼'
			if y == 0 && x == 0 {
				char = '┌'
			} else if y == 0 && x == grid.Width {
				char = '┐'
			} else if y == grid.Height && x == 0 {
				char = '└'
			} else if y == grid.Height && x == grid.Width {
				char = '┘'
			} else if y == 0 {
				char = '┬'
			} else if y == grid.Height {
				char = '┴'
			} else if x == 0 {
				char = '├'
			} else if x == grid.Width {
				char = '┤'
			}

			screen.SetContent(drX, drY, char, nil, styleBorder)

			if x < grid.Width {
				for i := 1; i < cellW; i++ {
					screen.SetContent(drX+i, drY, '─', nil, styleBorder)
				}
			}
			if y < grid.Height {
				for row := 1; row < cellH; row++ {
					screen.SetContent(drX, drY+row, '│', nil, styleBorder)
				}
			}
		}
	}

	// Draw Contents
	hlStartX, hlStartY, hlEndX, hlEndY := getHighlightBounds(state)
	dir := state.Cursor.Direction

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			cell := grid.Cells[y][x]
			drX := startX + x*cellW + 1

			// Clue numbers in top border
			if cell.Number > 0 {
				numStr := strconv.Itoa(cell.Number)
				for i, r := range numStr {
					if i < cellW-1 {
						screen.SetContent(startX+x*cellW+1+i, startY+y*cellH, r, nil, tcell.StyleDefault.Foreground(ColorBorder))
					}
				}
			}

			fgColor := ColorText
			if strings.Contains(state.Mode, "chk") || strings.Contains(state.Mode, "check") || strings.Contains(state.Mode, "tools") {
				fgColor = tcell.ColorWhite
				if cell.Value != 0 {
					if cell.CheckedCorrect {
						fgColor = tcell.ColorGreen
					} else {
						for _, w := range cell.WrongGuesses {
							if w == cell.Value {
								fgColor = tcell.ColorRed
								break
							}
						}
					}
				}
			}

			style := tcell.StyleDefault.Background(ColorBg).Foreground(fgColor)
			char := ' '

			if cell.IsBlack {
				style = tcell.StyleDefault.Background(ColorBg).Foreground(ColorBlackSq)
				char = '■'
			} else {
				isHlWord := false
				if dir == puzzle.DirAcross && y == state.Cursor.Y && x >= hlStartX && x <= hlEndX {
					isHlWord = true
				} else if dir == puzzle.DirDown && x == state.Cursor.X && y >= hlStartY && y <= hlEndY {
					isHlWord = true
				}

				hlFgColor := fgColor
				if fgColor == ColorText { // preserve normal highlight colors if not checked yet or not in check mode
					if isHlWord {
						hlFgColor = tcell.ColorWhite
					} // Across/Down text default
				}

				if x == state.Cursor.X && y == state.Cursor.Y {
					if fgColor != tcell.ColorWhite && fgColor != ColorText {
						style = tcell.StyleDefault.Background(ColorHighlight).Foreground(fgColor)
					} else {
						style = tcell.StyleDefault.Background(ColorHighlight).Foreground(ColorHlText)
					}
				} else if isHlWord {
					if dir == puzzle.DirAcross {
						style = tcell.StyleDefault.Background(ColorAcrossHl).Foreground(hlFgColor)
					} else {
						style = tcell.StyleDefault.Background(ColorDownHl).Foreground(hlFgColor)
					}
				}

				if cell.Value != 0 {
					char = rune(cell.Value)
				} else if isHlWord && !(x == state.Cursor.X && y == state.Cursor.Y) {
					char = '_'
				}
			}

			if !cell.IsBlack && state.Anagram.Active {
				ana := state.Anagram
				isAnaCell := false
				idx := -1

				if ana.Direction == puzzle.DirAcross && y == ana.StartY && x >= ana.StartX && x < ana.StartX+ana.Length {
					isAnaCell = true
					idx = x - ana.StartX
				} else if ana.Direction == puzzle.DirDown && x == ana.StartX && y >= ana.StartY && y < ana.StartY+ana.Length {
					isAnaCell = true
					idx = y - ana.StartY
				}

				if isAnaCell {
					cVal := ana.Letters[idx]
					if cVal != 0 {
						char = rune(cVal)
					} else {
						char = '_'
					}

					if idx == ana.CursorIdx {
						style = tcell.StyleDefault.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
					} else if ana.Locked[idx] {
						style = tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)
					} else {
						style = tcell.StyleDefault.Background(tcell.ColorDarkOrchid).Foreground(tcell.ColorWhite)
					}
				}
			}

			// Fill all content rows of the cell
			for row := 1; row < cellH; row++ {
				drY := startY + y*cellH + row
				for i := 0; i < cellW-1; i++ {
					if row == cellH/2 && i == (cellW-1)/2 {
						screen.SetContent(drX+i, drY, char, nil, style)
					} else {
						screen.SetContent(drX+i, drY, ' ', nil, style)
					}
				}
			}
		}
	}
}

// wrapText word-wraps a string to fit within maxWidth, returning lines.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	remaining := text

	for len(remaining) > maxWidth {
		splitIdx := maxWidth
		for i := maxWidth; i > 0; i-- {
			if remaining[i] == ' ' || remaining[i] == '-' {
				splitIdx = i
				break
			}
		}
		lines = append(lines, remaining[:splitIdx])
		remaining = strings.TrimSpace(remaining[splitIdx:])
	}
	if len(remaining) > 0 {
		lines = append(lines, remaining)
	}
	return lines
}

// drawClueBox returns the number of content lines drawn (for status bar positioning)
func drawClueBox(screen tcell.Screen, state *engine.GameState) int {
	if state.Puzzle == nil {
		return 0
	}

	grid := state.Puzzle.Grid
	startY := 2 + grid.Height*cellH + 1
	startX := 1
	width := gridPixelWidth(grid)

	// Find the current clue
	cx, cy := state.Cursor.X, state.Cursor.Y
	dir := state.Cursor.Direction

	bx, by := cx, cy
	if dir == puzzle.DirAcross {
		for bx > 0 && !grid.GetCell(bx-1, cy).IsBlack {
			bx--
		}
	} else {
		for by > 0 && !grid.GetCell(cx, by-1).IsBlack {
			by--
		}
	}

	cnum := grid.GetCell(bx, by).Number
	clueText := ""

	for _, c := range state.Puzzle.Clues {
		if c.Number == cnum && c.Direction == dir {
			clueText = c.Text
			break
		}
	}

	if clueText == "" {
		return 0 // No clue exists for this direction — don't render the box
	}

	// Word wrap the clue text to fit inside the box
	availWidth := width - 4
	clueLines := wrapText(clueText, availWidth)
	numLines := len(clueLines)
	if numLines < 1 {
		numLines = 1
	}

	// Box height: 1 (top border) + numLines (content) + 1 (bottom border)
	boxHeight := numLines + 2

	borderStyle := tcell.StyleDefault.Foreground(ColorClueBorder).Background(ColorBg)

	// Top border
	screen.SetContent(startX, startY, '┌', nil, borderStyle)
	screen.SetContent(startX+width-1, startY, '┐', nil, borderStyle)
	for x := startX + 1; x < startX+width-1; x++ {
		screen.SetContent(x, startY, '─', nil, borderStyle)
	}

	// Side borders + content lines
	for i := 0; i < numLines; i++ {
		screen.SetContent(startX, startY+1+i, '│', nil, borderStyle)
		screen.SetContent(startX+width-1, startY+1+i, '│', nil, borderStyle)
	}

	// Bottom border
	screen.SetContent(startX, startY+boxHeight-1, '└', nil, borderStyle)
	screen.SetContent(startX+width-1, startY+boxHeight-1, '┘', nil, borderStyle)
	for x := startX + 1; x < startX+width-1; x++ {
		screen.SetContent(x, startY+boxHeight-1, '─', nil, borderStyle)
	}

	// Draw Clue Header into top border — color by direction
	dirColor := ColorAcrossClueDir
	if dir == puzzle.DirDown {
		dirColor = ColorDownClueDir
	}
	dirStyle := tcell.StyleDefault.Foreground(dirColor).Background(ColorBg)
	headerText := fmt.Sprintf(" %d %s ", cnum, strings.ToLower(string(dir)))
	drawString(screen, startX+1, startY, headerText, dirStyle)

	// Draw Clue text lines
	textStyle := tcell.StyleDefault.Foreground(ColorClueText).Background(ColorBg)
	for i, line := range clueLines {
		drawString(screen, startX+2, startY+1+i, line, textStyle)
	}

	return boxHeight
}

func drawAllCluesBox(screen tcell.Screen, state *engine.GameState) int {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return 0
	}

	grid := state.Puzzle.Grid
	startY := 2 + grid.Height*cellH + 1
	startX := 1
	w, h := screen.Size()

	var counterLines int
	if strings.Contains(state.Mode, "chk") || strings.Contains(state.Mode, "check") || strings.Contains(state.Mode, "tools") {
		counterLines = 1
	}
	statusLines := 1

	maxBoxHeight := h - startY - counterLines - statusLines - 1
	if maxBoxHeight < 4 {
		return drawClueBox(screen, state) // Not enough space, fallback
	}

	// Active Clue Detection
	cx, cy := state.Cursor.X, state.Cursor.Y
	dir := state.Cursor.Direction

	bx, by := cx, cy
	if dir == puzzle.DirAcross {
		for bx > 0 && !grid.GetCell(bx-1, cy).IsBlack {
			bx--
		}
	} else {
		for by > 0 && !grid.GetCell(cx, by-1).IsBlack {
			by--
		}
	}

	activeCell := grid.GetCell(bx, by)
	activeNum := 0
	if activeCell != nil {
		activeNum = activeCell.Number
	}

	// Filter Clues
	var across []puzzle.Clue
	var down []puzzle.Clue
	for _, c := range state.Puzzle.Clues {
		if c.Direction == puzzle.DirAcross {
			across = append(across, c)
		} else {
			down = append(down, c)
		}
	}

	// Determine dimensions
	boxWidth := w - 2
	if boxWidth < 20 {
		boxWidth = 20
	}

	colWidth := (boxWidth - 1) / 2

	borderStyle := tcell.StyleDefault.Foreground(ColorClueBorder).Background(ColorBg)

	// Draw Box borders
	screen.SetContent(startX, startY, '┌', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY, '┐', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY, '─', nil, borderStyle)
	}

	// (Removed CLUES Header Badge)
	for i := 1; i < maxBoxHeight-1; i++ {
		screen.SetContent(startX, startY+i, '│', nil, borderStyle)
		screen.SetContent(startX+colWidth, startY+i, '│', nil, borderStyle)
		screen.SetContent(startX+boxWidth-1, startY+i, '│', nil, borderStyle)
	}

	screen.SetContent(startX, startY+maxBoxHeight-1, '└', nil, borderStyle)
	screen.SetContent(startX+colWidth, startY+maxBoxHeight-1, '┴', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY+maxBoxHeight-1, '┘', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		if x == startX+colWidth {
			continue
		}
		screen.SetContent(x, startY+maxBoxHeight-1, '─', nil, borderStyle)
	}

	screen.SetContent(startX, startY+2, '├', nil, borderStyle)
	screen.SetContent(startX+colWidth, startY+2, '┼', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY+2, '┤', nil, borderStyle)
	screen.SetContent(startX+colWidth, startY, '┬', nil, borderStyle)

	for x := startX + 1; x < startX+boxWidth-1; x++ {
		if x == startX+colWidth {
			continue
		}
		screen.SetContent(x, startY+2, '─', nil, borderStyle)
	}

	// Inner headers (Across, Down)
	acrossHeaderStyle := tcell.StyleDefault.Foreground(ColorAcrossClueDir).Background(ColorBg)
	downHeaderStyle := tcell.StyleDefault.Foreground(ColorDownClueDir).Background(ColorBg)
	drawString(screen, startX+2, startY+1, "Across", acrossHeaderStyle)
	drawString(screen, startX+colWidth+2, startY+1, "Down", downHeaderStyle)

	// Draw Clues Lists
	maxLines := maxBoxHeight - 4

	longestList := len(across)
	if len(down) > longestList {
		longestList = len(down)
	}
	maxScroll := longestList - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.ClueScrollOffset > maxScroll {
		state.ClueScrollOffset = maxScroll
	}
	offset := state.ClueScrollOffset

	drawList := func(cluelist []puzzle.Clue, colStartX int, activeDir puzzle.Direction) {
		baseStyle := tcell.StyleDefault.Foreground(ColorClueText).Background(ColorBg)
		hlStyle := tcell.StyleDefault.Foreground(ColorAcrossText).Background(ColorAcrossHl)
		if activeDir == puzzle.DirDown {
			hlStyle = tcell.StyleDefault.Foreground(ColorDownText).Background(ColorDownHl)
		}

		for i := 0; i < maxLines && i+offset < len(cluelist); i++ {
			c := cluelist[i+offset]
			txt := fmt.Sprintf("%d. %s", c.Number, c.Text)
			txt = runewidth.Truncate(txt, colWidth-3, "...")

			st := baseStyle
			if dir == activeDir && c.Number == activeNum {
				st = hlStyle
				txt = runewidth.FillRight(txt, colWidth-3)
			}
			drawString(screen, colStartX+2, startY+3+i, txt, st)
		}
	}

	drawList(across, startX, puzzle.DirAcross)
	drawList(down, startX+colWidth, puzzle.DirDown)

	return maxBoxHeight
}

func drawCounter(screen tcell.Screen, state *engine.GameState, offsetLines int) int {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return 0
	}
	grid := state.Puzzle.Grid
	startY := 2 + grid.Height*cellH + 1 + offsetLines
	startX := 1

	solveAcross := 0
	totalAcross := 0
	solveDown := 0
	totalDown := 0

	for _, c := range state.Puzzle.Clues {
		if c.Direction == puzzle.DirAcross {
			totalAcross++
			solved := true
			for i := 0; i < c.Length; i++ {
				cell := grid.GetCell(c.StartX+i, c.StartY)
				if cell == nil || !cell.CheckedCorrect {
					solved = false
					break
				}
			}
			if solved {
				solveAcross++
			}
		} else {
			totalDown++
			solved := true
			for i := 0; i < c.Length; i++ {
				cell := grid.GetCell(c.StartX, c.StartY+i)
				if cell == nil || !cell.CheckedCorrect {
					solved = false
					break
				}
			}
			if solved {
				solveDown++
			}
		}
	}

	text := fmt.Sprintf(" Across: %d/%d | Down: %d/%d ", solveAcross, totalAcross, solveDown, totalDown)
	style := tcell.StyleDefault.Background(ColorBg).Foreground(tcell.ColorDarkGray)
	drawString(screen, startX, startY, text, style)

	return 1
}

func drawStatus(screen tcell.Screen, state *engine.GameState, offsetLines int) {
	if state.Puzzle == nil {
		return
	}
	grid := state.Puzzle.Grid
	startY := 2 + grid.Height*cellH + 1 + offsetLines
	startX := 1
	width := gridPixelWidth(grid)

	style := tcell.StyleDefault.Background(ColorStatusBg).Foreground(ColorStatusFg)
	var statusText string

	if state.StatusMsg != "" && time.Now().Before(state.StatusExp) {
		statusText = state.StatusMsg
		if state.StatusLevel == "warn" {
			style = tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
		} else {
			style = tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorWhite)
		}
	} else if state.Anagram.Active {
		statusText = " ANAGRAM TOOL | ARROWS: move | L: lock | SPACE: shuffle | ENTER: commit | ESC: cancel "
		style = tcell.StyleDefault.Background(tcell.ColorDarkOrchid).Foreground(tcell.ColorWhite)
	} else if state.GotoMode {
		statusText = fmt.Sprintf(" GOTO CLUE: %s_ (ENTER to jump, ESC to cancel) ", state.GotoBuffer)
		style = tcell.StyleDefault.Background(ColorAcrossHl).Foreground(tcell.ColorWhite)
	} else {
		mod := "^"
		if runtime.GOOS != "darwin" {
			mod = "A-"
		}

		var parts []string
		parts = append(parts, mod+"G:Go", mod+"C:Clues", "TAB:Dir")

		if strings.HasPrefix(state.Mode, "blind") {
			parts = append(parts, mod+"S:Sub")
		}

		if strings.Contains(state.Mode, "chk") || strings.Contains(state.Mode, "check") || strings.Contains(state.Mode, "tools") {
			parts = append(parts, mod+"W:ChkWd", mod+"E:ChkAll", mod+"T:RevWd", mod+"Y:RevAll")
		}

		if state.IsDuel {
			parts = append(parts, mod+"Q:Resign", mod+"D:Draw")
		}

		parts = append(parts, mod+"R:Reset")

		if strings.Contains(state.Mode, "tools") {
			parts = append(parts, mod+"A:Ana")
		}

		parts = append(parts, "ESC:Quit")

		statusText = " " + strings.Join(parts, " | ") + " "
	}

	for x := startX; x < startX+width; x++ {
		screen.SetContent(x, startY, ' ', nil, style)
	}
	drawString(screen, startX, startY, statusText, style)
}

func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for i, r := range s {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

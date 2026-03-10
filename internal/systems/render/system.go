package rendersystem

import (
	"crossterm/internal/engine"
	"crossterm/internal/puzzle"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
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

	// Across highlight = teal/cyan family
	ColorAcrossHl   = tcell.NewRGBColor(0, 100, 120)
	ColorAcrossText = tcell.ColorWhite

	// Down highlight = purple/magenta family
	ColorDownHl   = tcell.NewRGBColor(80, 40, 120)
	ColorDownText = tcell.ColorWhite

	ColorHeaderBg = tcell.ColorSteelBlue
	ColorHeaderFg = tcell.ColorBlack

	ColorClueBoxBg  = tcell.ColorDefault
	ColorClueBorder = tcell.ColorTeal
	ColorClueText   = tcell.ColorLightGreen

	ColorAcrossClueDir = tcell.NewRGBColor(0, 200, 200)
	ColorDownClueDir   = tcell.NewRGBColor(180, 100, 220)

	ColorStatusBg = tcell.ColorSteelBlue
	ColorStatusFg = tcell.ColorBlack
)

type RenderSystem struct {
	Screen   tcell.Screen
	EventBus *engine.EventBus
	State    *engine.GameState
	initTime time.Time
}

func NewRenderSystem(screen tcell.Screen, eb *engine.EventBus, state *engine.GameState) *RenderSystem {
	return &RenderSystem{
		Screen:   screen,
		EventBus: eb,
		State:    state,
		initTime: time.Now(),
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

	drawHeader(s.Screen, s.State, s.initTime)
	drawGrid(s.Screen, s.State)
	clueLines := drawClueBox(s.Screen, s.State)
	drawStatus(s.Screen, s.State, clueLines)

	s.Screen.Show()
}

func drawHeader(screen tcell.Screen, state *engine.GameState, initTime time.Time) {
	w, _ := screen.Size()

	style := tcell.StyleDefault.Background(ColorHeaderBg).Foreground(ColorHeaderFg)
	title := fmt.Sprintf(" cryptic: %s - %s ", state.Puzzle.Title, state.Puzzle.Author)

	for x := 0; x < w; x++ {
		screen.SetContent(x, 0, ' ', nil, style)
	}

	drawString(screen, 0, 0, title, style)

	if !strings.HasPrefix(state.Mode, "not_timed") {
		elapsed := time.Since(initTime)
		timer := fmt.Sprintf(" %02d:%02d:%02d ", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
		drawString(screen, w-len(timer), 0, timer, style)
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

			style := tcell.StyleDefault.Background(ColorBg).Foreground(ColorText)
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

				if x == state.Cursor.X && y == state.Cursor.Y {
					style = tcell.StyleDefault.Background(ColorHighlight).Foreground(ColorHlText)
				} else if isHlWord {
					if dir == puzzle.DirAcross {
						style = tcell.StyleDefault.Background(ColorAcrossHl).Foreground(ColorAcrossText)
					} else {
						style = tcell.StyleDefault.Background(ColorDownHl).Foreground(ColorDownText)
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

func drawStatus(screen tcell.Screen, state *engine.GameState, clueBoxHeight int) {
	if state.Puzzle == nil {
		return
	}
	grid := state.Puzzle.Grid
	startY := 2 + grid.Height*cellH + 1 + clueBoxHeight
	startX := 1
	width := gridPixelWidth(grid)

	style := tcell.StyleDefault.Background(ColorStatusBg).Foreground(ColorStatusFg)

	var statusText string
	if state.Anagram.Active {
		statusText = " ANAGRAM TOOL | ARROWS: move | L: lock | SPACE: shuffle | ENTER: commit | ESC: cancel "
		style = tcell.StyleDefault.Background(tcell.ColorDarkOrchid).Foreground(tcell.ColorWhite)
	} else if state.GotoMode {
		statusText = fmt.Sprintf(" GOTO CLUE: %s_ (ENTER to jump, ESC to cancel) ", state.GotoBuffer)
		style = tcell.StyleDefault.Background(ColorAcrossHl).Foreground(tcell.ColorWhite)
	} else {
		statusText = " NORMAL | ^G: goto | TAB: dir | ARROWS: move | PgUp/Dn | ^R: reset "
		if strings.Contains(state.Mode, "tools") {
			statusText += "| ^A: anagram "
		}
		statusText += "| ESC: quit "
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

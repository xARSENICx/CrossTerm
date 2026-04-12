package puzzlesystem

import (
	"crossterm/internal/engine"
	"crossterm/internal/puzzle"
	"math/rand"
	"strconv"
	"strings"
	
	"github.com/gdamore/tcell/v2"
	"time"
	"unicode"
)

type PuzzleSystem struct {
	EventBus *engine.EventBus
	State    *engine.GameState
}

func NewPuzzleSystem(eb *engine.EventBus, state *engine.GameState) *PuzzleSystem {
	return &PuzzleSystem{
		EventBus: eb,
		State:    state,
	}
}

func (s *PuzzleSystem) Run() {
	keySub := s.EventBus.Subscribe(engine.EventKeyPress)
	mouseSub := s.EventBus.Subscribe(engine.EventMouseScroll)
	stopSub := s.EventBus.Subscribe(engine.EventShutdown)

	for {
		select {
		case <-stopSub:
			return
		case evt := <-keySub:
			if kEvent, ok := evt.Payload.(engine.KeyEventPayload); ok {
				s.handleKey(kEvent)
			}
		case evt := <-mouseSub:
			if btnMask, ok := evt.Payload.(tcell.ButtonMask); ok {
				s.handleMouse(btnMask)
			}
		}
	}
}

func (s *PuzzleSystem) handleKey(ev engine.KeyEventPayload) {
	if s.State.Puzzle == nil {
		return
	}
	c := ev.Rune
	k := ev.Key

	// If finished, only allow Menu (Escape), Exit (Ctrl+Q) and Clue Toggles (Ctrl-C)
	if s.State.IsFinished {
		if k == tcell.KeyEscape {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventReturnToMenu,
			})
			return
		}
		if k == tcell.KeyCtrlQ || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'q') {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventQuit,
			})
			return
		}
		if k == tcell.KeyCtrlC || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'c') {
			s.State.ShowAllClues = !s.State.ShowAllClues
			s.EventBus.Publish(engine.Event{
				Type: engine.EventStateUpdate,
			})
		}
		return
	}

	// Universal Exit check
	if k == tcell.KeyCtrlQ || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'q') {
		if s.State.IsDuel {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventResign,
			})
		}
		s.EventBus.Publish(engine.Event{
			Type: engine.EventQuit,
		})
		return
	}

	// Universal Menu check - but only if NOT in a sub-mode that uses ESC for cancel
	if !s.State.Anagram.Active && !s.State.GotoMode && k == tcell.KeyEscape {
		s.EventBus.Publish(engine.Event{
			Type: engine.EventReturnToMenu,
		})
		return
	}

	// Undo/Redo checks
	if k == tcell.KeyCtrlZ || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'z') {
		s.handleUndo()
		s.EventBus.Publish(engine.Event{
			Type: engine.EventStateUpdate,
		})
		return
	}
	if k == tcell.KeyCtrlY || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'y') {
		s.handleRedo()
		s.EventBus.Publish(engine.Event{
			Type: engine.EventStateUpdate,
		})
		return
	}

	// Pause toggle can happen regardless of other modes, but only in timed modes
	if k == tcell.KeyCtrlP || (ev.Modifiers&tcell.ModAlt != 0 && unicode.ToLower(c) == 'p') {
		if strings.Contains(s.State.Mode, "timed") {
			s.togglePause()
			s.EventBus.Publish(engine.Event{
				Type: engine.EventStateUpdate,
			})
			return
		}
	}

	if s.State.IsPaused {
		return
	}

	modified := false

	if s.State.Anagram.Active {
		if k == tcell.KeyEnter {
			s.commitAnagram()
			modified = true
		} else if k == tcell.KeyEscape {
			s.State.Anagram.Active = false
			modified = true
		} else if k == tcell.KeyLeft || k == tcell.KeyUp {
			if s.State.Anagram.CursorIdx > 0 {
				s.State.Anagram.CursorIdx--
				modified = true
			}
		} else if k == tcell.KeyRight || k == tcell.KeyDown {
			if s.State.Anagram.CursorIdx < s.State.Anagram.Length-1 {
				s.State.Anagram.CursorIdx++
				modified = true
			}
		} else if c == 'l' || c == 'L' {
			s.State.Anagram.Locked[s.State.Anagram.CursorIdx] = !s.State.Anagram.Locked[s.State.Anagram.CursorIdx]
			modified = true
		} else if c == ' ' {
			s.shuffleAnagram()
			modified = true
		} else if k == tcell.KeyBackspace || k == tcell.KeyBackspace2 {
			if !s.State.Anagram.Locked[s.State.Anagram.CursorIdx] {
				s.State.Anagram.Letters[s.State.Anagram.CursorIdx] = 0
				modified = true
			}
			if s.State.Anagram.CursorIdx > 0 {
				s.State.Anagram.CursorIdx--
			}
		} else if unicode.IsLetter(c) {
			if !s.State.Anagram.Locked[s.State.Anagram.CursorIdx] {
				s.State.Anagram.Letters[s.State.Anagram.CursorIdx] = byte(unicode.ToUpper(c))
				modified = true
			}
			if s.State.Anagram.CursorIdx < s.State.Anagram.Length-1 {
				s.State.Anagram.CursorIdx++
			}
		}

		if modified {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventStateUpdate,
			})
		}
		return
	}

	if s.State.GotoMode {
		if k == tcell.KeyEnter {
			s.submitGoto()
			modified = true
		} else if k == tcell.KeyEscape {
			s.State.GotoMode = false
			s.State.GotoBuffer = ""
			modified = true
		} else if k == tcell.KeyBackspace || k == tcell.KeyBackspace2 {
			if len(s.State.GotoBuffer) > 0 {
				s.State.GotoBuffer = s.State.GotoBuffer[:len(s.State.GotoBuffer)-1]
				modified = true
			}
		} else if unicode.IsDigit(c) {
			s.State.GotoBuffer += string(c)
			modified = true
		}
		
		if modified {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventStateUpdate,
			})
		}
		return
	}

	// Handle directional movement
	switch k {
	case tcell.KeyUp:
		s.moveCursor(0, -1)
		modified = true
	case tcell.KeyDown:
		s.moveCursor(0, 1)
		modified = true
	case tcell.KeyLeft:
		s.moveCursor(-1, 0)
		modified = true
	case tcell.KeyRight:
		s.moveCursor(1, 0)
		modified = true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.handleBackspace()
		modified = true
	case tcell.KeyEnter, tcell.KeyTab:
		// If status message is active, Enter dismisses it
		if s.State.StatusMsg != "" && time.Now().Before(s.State.StatusExp) && k == tcell.KeyEnter {
			s.State.StatusMsg = ""
			modified = true
		} else {
			// Toggle direction only if the other direction has a valid clue
			newDir := puzzle.DirDown
			if s.State.Cursor.Direction == puzzle.DirDown {
				newDir = puzzle.DirAcross
			}
			if s.hasClueInDir(s.State.Cursor.X, s.State.Cursor.Y, newDir) {
				s.State.Cursor.Direction = newDir
				modified = true
			}
		}
	case tcell.KeyCtrlR:
		s.handleReset()
		modified = true
	case tcell.KeyPgUp:
		s.handlePgUp()
		modified = true
	case tcell.KeyPgDn:
		s.handlePgDn()
		modified = true
	case tcell.KeyCtrlG:
		s.State.GotoMode = true
		s.State.GotoBuffer = ""
		modified = true
	case tcell.KeyCtrlA:
		if strings.Contains(s.State.Mode, "tools") && !s.State.Anagram.Active {
			s.enterAnagramMode()
			modified = true
		}
	case tcell.KeyCtrlW:
		if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
			s.handleCheckWord()
			modified = true
		}
	case tcell.KeyCtrlE:
		if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
			s.handleCheckAll()
			modified = true
		}
	case tcell.KeyCtrlT:
		if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
			s.handleRevealWord()
			modified = true
		}
	case tcell.KeyCtrlY:
		if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
			s.handleRevealAll()
			modified = true
		}
	case tcell.KeyCtrlS:
		s.EventBus.Publish(engine.Event{
			Type: engine.EventPuzzleSubmit,
		})
		modified = true
	// ESC and Ctrl+Q are handled above as universals now
	case tcell.KeyCtrlD:
		if s.State.IsDuel {
			s.EventBus.Publish(engine.Event{
				Type: engine.EventDrawOffer,
			})
			modified = true
		}
	case tcell.KeyCtrlC:
		s.State.ShowAllClues = !s.State.ShowAllClues
		modified = true
	default:
		// Check for Alt-key combinations (Alt as an alternative for Ctrl)
		if ev.Modifiers&tcell.ModAlt != 0 {
			switch unicode.ToLower(ev.Rune) {
			case 'g':
				s.State.GotoMode = true
				s.State.GotoBuffer = ""
				modified = true
			case 'r':
				s.handleReset()
				modified = true
			case 'a':
				if strings.Contains(s.State.Mode, "tools") && !s.State.Anagram.Active {
					s.enterAnagramMode()
					modified = true
				}
			case 'w':
				if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
					s.handleCheckWord()
					modified = true
				}
			case 'e':
				if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
					s.handleCheckAll()
					modified = true
				}
			case 't':
				if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
					s.handleRevealWord()
					modified = true
				}
			case 'y':
				if strings.Contains(s.State.Mode, "chk") || strings.Contains(s.State.Mode, "check") || strings.Contains(s.State.Mode, "tools") {
					s.handleRevealAll()
					modified = true
				}
			case 's':
				s.EventBus.Publish(engine.Event{
					Type: engine.EventPuzzleSubmit,
				})
				modified = true
			case 'd':
				if s.State.IsDuel {
					s.EventBus.Publish(engine.Event{
						Type: engine.EventDrawOffer,
					})
					modified = true
				}
			}
			break
		}

		// Check for letter input
		if unicode.IsLetter(c) {
			s.typeLetter(unicode.ToUpper(c))
			modified = true
		} else if c == ' ' {
			newDir := puzzle.DirDown
			if s.State.Cursor.Direction == puzzle.DirDown {
				newDir = puzzle.DirAcross
			}
			if s.hasClueInDir(s.State.Cursor.X, s.State.Cursor.Y, newDir) {
				s.State.Cursor.Direction = newDir
				modified = true
			}
		}
	}

	if modified {
		// Emit state update event to redraw UI
		s.EventBus.Publish(engine.Event{
			Type: engine.EventStateUpdate,
		})
	}
}

func (s *PuzzleSystem) handleMouse(btns tcell.ButtonMask) {
	if !s.State.ShowAllClues {
		return
	}

	modified := false
	if btns&tcell.WheelUp != 0 {
		s.State.ClueScrollOffset--
		if s.State.ClueScrollOffset < 0 {
			s.State.ClueScrollOffset = 0
		}
		modified = true
	} else if btns&tcell.WheelDown != 0 {
		s.State.ClueScrollOffset++
		modified = true
	}

	if modified {
		s.EventBus.Publish(engine.Event{
			Type: engine.EventStateUpdate,
		})
	}
}

func (s *PuzzleSystem) typeLetter(char rune) {
	s.captureHistory()
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y

	cell := grid.GetCell(cx, cy)
	if cell != nil && !cell.IsBlack {
		cell.Value = byte(char)
		cell.CheckedCorrect = false
		if s.State.IsCollab {
			cell.TypedBy = 1
		}
		if cell.WasChecked {
			s.checkCell(cell)
		}
		s.EventBus.Publish(engine.Event{
			Type:    engine.EventCellTyped,
			Payload: []int{cx, cy}, // Pass coords so network system can fetch the exact cell
		})
	}
	s.advanceCursor(1)
}

func (s *PuzzleSystem) handleBackspace() {
	s.captureHistory()
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y

	cell := grid.GetCell(cx, cy)
	if cell != nil && !cell.IsBlack {
		cell.CheckedCorrect = false
		if cell.Value == 0 {
			// If empty, move back first, then delete
			s.advanceCursor(-1)
			prev := grid.GetCell(s.State.Cursor.X, s.State.Cursor.Y)
			if prev != nil && !prev.IsBlack {
				prev.Value = 0
				prev.CheckedCorrect = false
				if s.State.IsCollab {
					prev.TypedBy = 1
				}
				s.EventBus.Publish(engine.Event{Type: engine.EventCellTyped, Payload: []int{s.State.Cursor.X, s.State.Cursor.Y}})
			}
		} else {
			// Delete current
			cell.Value = 0
			if s.State.IsCollab {
				cell.TypedBy = 1
			}
			s.EventBus.Publish(engine.Event{Type: engine.EventCellTyped, Payload: []int{cx, cy}})
		}
	} else {
		s.advanceCursor(-1)
	}
}

func (s *PuzzleSystem) moveCursor(dx, dy int) {
	newX := s.State.Cursor.X + dx
	newY := s.State.Cursor.Y + dy
	s.moveTo(newX, newY)
}

func (s *PuzzleSystem) advanceCursor(step int) {
	dx, dy := 0, 0
	if s.State.Cursor.Direction == puzzle.DirAcross {
		dx = step
	} else {
		dy = step
	}

	// Try moving, but don't wrap out of grid normally
	for i := 0; i < 50; i++ { // limit iterations just in case
		newX := s.State.Cursor.X + dx
		newY := s.State.Cursor.Y + dy

		cell := s.State.Puzzle.Grid.GetCell(newX, newY)
		if cell == nil {
			return // edge of grid
		}

		s.State.Cursor.X = newX
		s.State.Cursor.Y = newY

		if !cell.IsBlack {
			return // found valid square
		}
	}
}

func (s *PuzzleSystem) moveTo(x, y int) {
	cell := s.State.Puzzle.Grid.GetCell(x, y)
	if cell != nil && !cell.IsBlack {
		s.State.Cursor.X = x
		s.State.Cursor.Y = y
	}
}

func (s *PuzzleSystem) handleReset() {
	s.captureHistory()
	grid := s.State.Puzzle.Grid
	if grid == nil {
		return
	}
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			if !grid.Cells[y][x].IsBlack {
				grid.Cells[y][x].Value = 0
			}
		}
	}
}

func (s *PuzzleSystem) hasClueInDir(cx, cy int, dir puzzle.Direction) bool {
	grid := s.State.Puzzle.Grid
	// Walk back to find the start of the word in the given direction
	bx, by := cx, cy
	if dir == puzzle.DirAcross {
		for bx > 0 && !grid.GetCell(bx-1, cy).IsBlack {
			bx--
		}
		// Must have at least 2 cells across
		if bx+1 >= grid.Width || grid.GetCell(bx+1, cy).IsBlack {
			return false
		}
	} else {
		for by > 0 && !grid.GetCell(cx, by-1).IsBlack {
			by--
		}
		// Must have at least 2 cells down
		if by+1 >= grid.Height || grid.GetCell(cx, by+1).IsBlack {
			return false
		}
	}

	cnum := grid.GetCell(bx, by).Number
	if cnum == 0 {
		return false
	}

	for _, c := range s.State.Puzzle.Clues {
		if c.Number == cnum && c.Direction == dir {
			return true
		}
	}
	return false
}

func (s *PuzzleSystem) enterAnagramMode() {
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y
	dir := s.State.Cursor.Direction

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

	length := 0
	nx, ny := bx, by
	if dir == puzzle.DirAcross {
		for nx < grid.Width && !grid.GetCell(nx, ny).IsBlack {
			length++
			nx++
		}
	} else {
		for ny < grid.Height && !grid.GetCell(nx, ny).IsBlack {
			length++
			ny++
		}
	}

	if length <= 1 {
		return // Can't anagram 1 letter
	}

	letters := make([]byte, length)
	locked := make([]bool, length)
	cursorIdx := 0

	nx, ny = bx, by
	for i := 0; i < length; i++ {
		if dir == puzzle.DirAcross {
			letters[i] = grid.GetCell(nx+i, ny).Value
			if nx+i == cx {
				cursorIdx = i
			}
		} else {
			letters[i] = grid.GetCell(nx, ny+i).Value
			if ny+i == cy {
				cursorIdx = i
			}
		}
	}

	s.State.Anagram = engine.AnagramState{
		Active:    true,
		StartX:    bx,
		StartY:    by,
		Direction: dir,
		Length:    length,
		Letters:   letters,
		Locked:    locked,
		CursorIdx: cursorIdx,
	}
}

func (s *PuzzleSystem) commitAnagram() {
	grid := s.State.Puzzle.Grid
	ana := s.State.Anagram
	nx, ny := ana.StartX, ana.StartY
	for i := 0; i < ana.Length; i++ {
		cell := grid.GetCell(nx, ny)
		if cell != nil {
			cell.Value = ana.Letters[i]
		}
		if ana.Direction == puzzle.DirAcross {
			nx++
		} else {
			ny++
		}
	}
	s.State.Anagram.Active = false
}

func (s *PuzzleSystem) shuffleAnagram() {
	ana := &s.State.Anagram
	var pool []byte
	var indices []int

	for i := 0; i < ana.Length; i++ {
		if !ana.Locked[i] && ana.Letters[i] != 0 {
			pool = append(pool, ana.Letters[i])
			indices = append(indices, i)
		}
	}

	if len(pool) <= 1 {
		return
	}

	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	for i, idx := range indices {
		ana.Letters[idx] = pool[i]
	}
}

func (s *PuzzleSystem) submitGoto() {
	s.State.GotoMode = false
	num, err := strconv.Atoi(s.State.GotoBuffer)
	s.State.GotoBuffer = ""
	if err != nil || num <= 0 {
		return
	}

	// Find the cell matching this clue number
	// Prefer matching current direction if possible, else jump to it and switch dir.
	currentDir := s.State.Cursor.Direction
	var found *puzzle.Clue
	var fallback *puzzle.Clue

	for i := range s.State.Puzzle.Clues {
		c := &s.State.Puzzle.Clues[i]
		if c.Number == num {
			if c.Direction == currentDir {
				found = c
			} else {
				fallback = c
			}
		}
	}

	target := found
	if target == nil {
		target = fallback
	}

	if target != nil {
		s.State.Cursor.X = target.StartX
		s.State.Cursor.Y = target.StartY
		s.State.Cursor.Direction = target.Direction
	}
}

func (s *PuzzleSystem) handlePgUp() {
	s.jumpClue(-1)
}

func (s *PuzzleSystem) handlePgDn() {
	s.jumpClue(1)
}

func (s *PuzzleSystem) jumpClue(offset int) {
	dir := s.State.Cursor.Direction
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y

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

	var dirClues []puzzle.Clue
	for _, c := range s.State.Puzzle.Clues {
		if c.Direction == dir {
			dirClues = append(dirClues, c)
		}
	}

	if len(dirClues) == 0 {
		return
	}

	idx := -1
	for i, c := range dirClues {
		if c.Number == cnum {
			idx = i
			break
		}
	}

	if idx != -1 {
		idx = (idx + offset) % len(dirClues)
		if idx < 0 {
			idx += len(dirClues)
		}
		target := dirClues[idx]
		s.State.Cursor.X = target.StartX
		s.State.Cursor.Y = target.StartY
	} else {
		// Just jump to first available clue in that direction
		target := dirClues[0]
		s.State.Cursor.X = target.StartX
		s.State.Cursor.Y = target.StartY
	}
}

func (s *PuzzleSystem) handleCheckWord() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	if !s.State.Puzzle.HasSolution {
		s.State.StatusMsg = " Puzzle doesn't have any solutions to check/reveal "
		s.State.StatusExp = time.Now().Add(5 * time.Second)
		s.State.StatusLevel = "error"
		return
	}
	s.captureHistory()
	s.State.CheckCount++
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y

	startX, endX := cx, cx
	startY, endY := cy, cy

	if s.State.Cursor.Direction == puzzle.DirAcross {
		for startX > 0 && !grid.GetCell(startX-1, cy).IsBlack {
			startX--
		}
		for endX < grid.Width-1 && !grid.GetCell(endX+1, cy).IsBlack {
			endX++
		}
	} else {
		for startY > 0 && !grid.GetCell(cx, startY-1).IsBlack {
			startY--
		}
		for endY < grid.Height-1 && !grid.GetCell(cx, endY+1).IsBlack {
			endY++
		}
	}

	for y := startY; y <= endY; y++ {
		for x := startX; x <= endX; x++ {
			s.checkCell(&grid.Cells[y][x])
		}
	}
}

func (s *PuzzleSystem) handleCheckAll() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	if !s.State.Puzzle.HasSolution {
		s.State.StatusMsg = " Puzzle doesn't have any solutions to check/reveal "
		s.State.StatusExp = time.Now().Add(5 * time.Second)
		s.State.StatusLevel = "error"
		return
	}
	s.captureHistory()
	s.State.CheckCount++
	grid := s.State.Puzzle.Grid
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			s.checkCell(&grid.Cells[y][x])
		}
	}
}

func (s *PuzzleSystem) checkCell(cell *puzzle.Cell) {
	if cell.IsBlack || cell.Value == 0 {
		return
	}
	cell.WasChecked = true
	cell.CheckedCorrect = false

	if cell.Value == cell.Solution {
		cell.CheckedCorrect = true
	} else {
		// Only add to WrongGuesses if not already there
		found := false
		for _, w := range cell.WrongGuesses {
			if w == cell.Value {
				found = true
				break
			}
		}
		if !found {
			cell.WrongGuesses = append(cell.WrongGuesses, cell.Value)
		}
	}
}

func (s *PuzzleSystem) handleRevealWord() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	if !s.State.Puzzle.HasSolution {
		s.State.StatusMsg = " Puzzle doesn't have any solutions to check/reveal "
		s.State.StatusExp = time.Now().Add(5 * time.Second)
		s.State.StatusLevel = "error"
		return
	}
	s.captureHistory()
	s.State.RevealCount++
	grid := s.State.Puzzle.Grid
	cx, cy := s.State.Cursor.X, s.State.Cursor.Y

	startX, endX := cx, cx
	startY, endY := cy, cy

	if s.State.Cursor.Direction == puzzle.DirAcross {
		for startX > 0 && !grid.GetCell(startX-1, cy).IsBlack {
			startX--
		}
		for endX < grid.Width-1 && !grid.GetCell(endX+1, cy).IsBlack {
			endX++
		}
	} else {
		for startY > 0 && !grid.GetCell(cx, startY-1).IsBlack {
			startY--
		}
		for endY < grid.Height-1 && !grid.GetCell(cx, endY+1).IsBlack {
			endY++
		}
	}

	for y := startY; y <= endY; y++ {
		for x := startX; x <= endX; x++ {
			s.revealCell(&grid.Cells[y][x])
		}
	}
}

func (s *PuzzleSystem) handleRevealAll() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	if !s.State.Puzzle.HasSolution {
		s.State.StatusMsg = " Puzzle doesn't have any solutions to check/reveal "
		s.State.StatusExp = time.Now().Add(5 * time.Second)
		s.State.StatusLevel = "error"
		return
	}
	s.captureHistory()
	s.State.RevealCount++
	grid := s.State.Puzzle.Grid
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			s.revealCell(&grid.Cells[y][x])
		}
	}
}

func (s *PuzzleSystem) revealCell(cell *puzzle.Cell) {
	if cell.IsBlack {
		return
	}
	cell.Value = cell.Solution
	cell.CheckedCorrect = true
	cell.WasChecked = true
}

func (s *PuzzleSystem) togglePause() {
	if s.State.IsPaused {
		// Resuming
		s.State.IsPaused = false
		s.State.TotalPausedTime += time.Since(s.State.PauseStartTime)
	} else {
		// Pausing
		s.State.IsPaused = true
		s.State.PauseStartTime = time.Now()
	}
}

func (s *PuzzleSystem) captureHistory() {
	s.State.UndoStack = append(s.State.UndoStack, s.captureCurrent())
	if len(s.State.UndoStack) > 100 {
		s.State.UndoStack = s.State.UndoStack[1:]
	}
	s.State.RedoStack = nil
}

func (s *PuzzleSystem) handleUndo() {
	if len(s.State.UndoStack) == 0 {
		return
	}

	// Capture current state for redo
	s.captureRedo()

	// Pop from undo
	entry := s.State.UndoStack[len(s.State.UndoStack)-1]
	s.State.UndoStack = s.State.UndoStack[:len(s.State.UndoStack)-1]

	s.applyHistory(entry)
}

func (s *PuzzleSystem) handleRedo() {
	if len(s.State.RedoStack) == 0 {
		return
	}

	// Capture current state for undo
	s.State.UndoStack = append(s.State.UndoStack, s.captureCurrent())
	if len(s.State.UndoStack) > 100 {
		s.State.UndoStack = s.State.UndoStack[1:]
	}

	// Pop from redo
	entry := s.State.RedoStack[len(s.State.RedoStack)-1]
	s.State.RedoStack = s.State.RedoStack[:len(s.State.RedoStack)-1]

	s.applyHistory(entry)
}

func (s *PuzzleSystem) captureRedo() {
	s.State.RedoStack = append(s.State.RedoStack, s.captureCurrent())
	if len(s.State.RedoStack) > 100 {
		s.State.RedoStack = s.State.RedoStack[1:]
	}
}

func (s *PuzzleSystem) captureCurrent() engine.HistoryEntry {
	grid := s.State.Puzzle.Grid
	entry := engine.HistoryEntry{
		CursorX:   s.State.Cursor.X,
		CursorY:   s.State.Cursor.Y,
		CursorDir: s.State.Cursor.Direction,
		Cells:     make([][]engine.CellSnapshot, grid.Height),
	}
	for y := 0; y < grid.Height; y++ {
		entry.Cells[y] = make([]engine.CellSnapshot, grid.Width)
		for x := 0; x < grid.Width; x++ {
			cell := &grid.Cells[y][x]
			entry.Cells[y][x] = engine.CellSnapshot{
				Value:          cell.Value,
				CheckedCorrect: cell.CheckedCorrect,
				WasChecked:     cell.WasChecked,
			}
		}
	}
	return entry
}

func (s *PuzzleSystem) applyHistory(entry engine.HistoryEntry) {
	grid := s.State.Puzzle.Grid
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			cell := &grid.Cells[y][x]
			snap := entry.Cells[y][x]
			cell.Value = snap.Value
			cell.CheckedCorrect = snap.CheckedCorrect
			cell.WasChecked = snap.WasChecked
		}
	}
	s.State.Cursor.X = entry.CursorX
	s.State.Cursor.Y = entry.CursorY
	s.State.Cursor.Direction = entry.CursorDir
}

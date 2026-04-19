package ui

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

var (
	ColorBg        = tcell.ColorDefault
	ColorText      = tcell.ColorLightGreen
	ColorTitle     = tcell.ColorTeal
	ColorHighlight = tcell.ColorBlack
	ColorHlBg      = tcell.ColorLightGreen
	ColorSub       = tcell.ColorWhite
)

type MenuOption struct {
	Text string
	Val  string
}

func drawStr(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	col := 0
	for _, r := range s {
		screen.SetContent(x+col, y, r, nil, style)
		col += runewidth.RuneWidth(r)
	}
}

// DrawMenu blocks and returns the selected index or -1 if escaped
func DrawMenu(screen tcell.Screen, title string, options []MenuOption) int {
	selected := 0
	for {
		screen.Clear()
		w, h := screen.Size()

		titleLines := strings.Split(title, "\n")
		startY := h/2 - len(options)/2 - len(titleLines) - 1

		for i, line := range titleLines {
			lineLen := runewidth.StringWidth(line)
			drawStr(screen, w/2-lineLen/2, startY+i, line, tcell.StyleDefault.Foreground(ColorTitle).Background(ColorBg))
		}

		maxWidth := 0
		for _, opt := range options {
			optLen := runewidth.StringWidth(opt.Text)
			if optLen > maxWidth {
				maxWidth = optLen
			}
		}
		// Add a little extra padding so it's a nice comfortable rectangle
		maxWidth += 4

		optStartY := startY + len(titleLines) + 1

		for i, opt := range options {
			style := tcell.StyleDefault.Foreground(ColorText).Background(ColorBg)
			
			// Centrally pad the text to the maxWidth
			optLen := runewidth.StringWidth(opt.Text)
			paddingTotal := maxWidth - optLen
			padLeft := paddingTotal / 2
			padRight := paddingTotal - padLeft
			paddedText := strings.Repeat(" ", padLeft) + opt.Text + strings.Repeat(" ", padRight)

			text := "  " + paddedText + "  "

			if i == selected {
				style = tcell.StyleDefault.Foreground(ColorHighlight).Background(ColorHlBg)
				text = "> " + paddedText + " <"
			}

			textLen := runewidth.StringWidth(text)
			drawStr(screen, w/2-textLen/2, optStartY+i*2, text, style)
		}

		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyUp {
				selected--
				if selected < 0 {
					selected = len(options) - 1
				}
			} else if ev.Key() == tcell.KeyDown {
				selected++
				if selected >= len(options) {
					selected = 0
				}
			} else if ev.Key() == tcell.KeyEnter {
				return selected
			} else if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return -1
			}
		}
	}
}

// DrawText blocks and displays a static message (for loading or waiting) until a key is pressed, or returns immediately if blocking is false.
func DrawText(screen tcell.Screen, text string, blocking bool) {
	screen.Clear()
	w, h := screen.Size()

	lines := strings.Split(text, "\n")
	startY := h/2 - len(lines)/2

	for i, line := range lines {
		lineLen := runewidth.StringWidth(line)
		drawStr(screen, w/2-lineLen/2, startY+i, line, tcell.StyleDefault.Foreground(ColorSub).Background(ColorBg))
	}
	screen.Show()

	if blocking {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyEnter {
					return
				}
			}
		}
	}
}

// DrawGoodbye displays a polished exit screen before the process terminates.
func DrawGoodbye(screen tcell.Screen) {
	screen.Clear()
	w, h := screen.Size()

	lines := []string{
		"╔══════════════════════════════════════════════════════╗",
		"║                                                      ║",
		"║               THANK YOU FOR PLAYING                  ║",
		"║                    CROSS-TERM                        ║",
		"║                                                      ║",
		"║          Your progress has been auto-saved.          ║",
		"║                                                      ║",
		"║          [ PRESS ANY KEY TO CLOSE WINDOW ]           ║",
		"║                                                      ║",
		"╚══════════════════════════════════════════════════════╝",
	}

	startY := h/2 - len(lines)/2
	for i, line := range lines {
		lineLen := runewidth.StringWidth(line)
		drawStr(screen, w/2-lineLen/2, startY+i, line, tcell.StyleDefault.Foreground(ColorTitle).Background(ColorBg))
	}
	screen.Show()

	for {
		ev := screen.PollEvent()
		if _, ok := ev.(*tcell.EventKey); ok {
			return
		}
	}
}

// DrawInput blocks and provides a single text input field.
func DrawInput(screen tcell.Screen, title, subtitle string, maxLength int) string {
	inputStr := ""
	for {
		screen.Clear()
		w, h := screen.Size()

		titleLines := strings.Split(title, "\n")
		startY := h/2 - 4 - len(titleLines)

		for i, line := range titleLines {
			lineLen := runewidth.StringWidth(line)
			drawStr(screen, w/2-lineLen/2, startY+i, line, tcell.StyleDefault.Foreground(ColorTitle).Background(ColorBg))
		}

		subLines := strings.Split(subtitle, "\n")
		for i, line := range subLines {
			lineLen := runewidth.StringWidth(line)
			drawStr(screen, w/2-lineLen/2, startY+len(titleLines)+1+i, line, tcell.StyleDefault.Foreground(ColorSub).Background(ColorBg))
		}

		displayInput := "> " + inputStr + " _"
		
		inputY := startY + len(titleLines) + len(subLines) + 3
		inputLen := runewidth.StringWidth(displayInput)
		drawStr(screen, w/2-inputLen/2, inputY, displayInput, tcell.StyleDefault.Foreground(ColorHlBg).Background(ColorBg))

		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEnter {
				if len(inputStr) > 0 {
					return strings.ToUpper(strings.TrimSpace(inputStr))
				}
			} else if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
				if len(inputStr) > 0 {
					inputStr = inputStr[:len(inputStr)-1]
				}
			} else if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return ""
			} else if ev.Rune() >= 32 && ev.Rune() <= 126 { // Only printable ASCII
				if maxLength == 0 || len(inputStr) < maxLength {
					inputStr += string(ev.Rune())
				}
			}
		}
	}
}

// DrawControls displays a grouped list of controls based on the OS.
func DrawControls(screen tcell.Screen) {
	mod := "Ctrl+"
	if runtime.GOOS != "darwin" {
		mod = "Alt+"
	}

	pgKey := "PgUp/Dn"
	if runtime.GOOS == "darwin" {
		pgKey = "PgUp/Dn (Fn+Up/Dn)"
	}

	groups := []struct {
		Name  string
		Keyts []string
	}{
		{" NAVIGATION ", []string{"ARROWS: Move Cursor", "TAB: Switch Direction", pgKey + ": Clue-by-Clue Jump"}},
		{" BOARD ", []string{"Letters: Type solution", "BACKSPACE: Delete Letter", "ENTER: Jump to grid/Sub (Blind)", fmt.Sprintf("%sG: Go to Clue #", mod), fmt.Sprintf("%sR: Reset Grid", mod)}},
		{" ASSISTANT ", []string{fmt.Sprintf("%sW: Check Word", mod), fmt.Sprintf("%sE: Check All", mod), fmt.Sprintf("%sT: Reveal Word", mod), fmt.Sprintf("%sY: Reveal All", mod)}},
		{" TOOLS ", []string{fmt.Sprintf("%sA: Anagram tool", mod), fmt.Sprintf("%sC: Show All Clues (Full Screen)", mod), "MOUSE SCROLL: Scroll Clue List"}},
		{" COMPETITIVE (Multiplayer) ", []string{fmt.Sprintf("%sS: Submit Puzzle", mod), fmt.Sprintf("%sD: Draw Offer", mod)}},
		{" SYSTEM ", []string{fmt.Sprintf("%sZ: Undo", mod), fmt.Sprintf("%sY: Redo", mod), fmt.Sprintf("%sP: Pause/Resume (Timed)", mod), fmt.Sprintf("%sQ: Exit / Resign", mod), "ESC: Back to Menu"}},
	}

	for {
		screen.Clear()
		w, h := screen.Size()

		title := " CROSS-TERM CONTROLS "
		titleStyle := tcell.StyleDefault.Foreground(ColorTitle).Background(ColorBg).Bold(true)
		drawStr(screen, (w-runewidth.StringWidth(title))/2, 2, title, titleStyle)

		startY := 5
		for _, g := range groups {
			headerStyle := tcell.StyleDefault.Foreground(tcell.ColorTeal).Background(ColorBg).Bold(true)
			drawStr(screen, 4, startY, g.Name, headerStyle)
			startY++
			for _, k := range g.Keyts {
				keyStyle := tcell.StyleDefault.Foreground(ColorText).Background(ColorBg)
				drawStr(screen, 6, startY, "• "+k, keyStyle)
				startY++
			}
			startY++ // spacing
		}

		backMsg := " [ PRESS ANY KEY TO GO BACK ] "
		drawStr(screen, (w-runewidth.StringWidth(backMsg))/2, h-2, backMsg, tcell.StyleDefault.Foreground(ColorSub).Background(ColorBg))

		screen.Show()

		ev := screen.PollEvent()
		switch ev.(type) {
		case *tcell.EventKey:
			return
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

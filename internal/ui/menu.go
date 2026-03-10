package ui

import (
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

// DrawInput blocks and provides a single text input field.
func DrawInput(screen tcell.Screen, title, subtitle string) string {
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
					return inputStr
				}
			} else if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
				if len(inputStr) > 0 {
					inputStr = inputStr[:len(inputStr)-1]
				}
			} else if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return ""
			} else if ev.Rune() != 0 {
				inputStr += string(ev.Rune())
			}
		}
	}
}

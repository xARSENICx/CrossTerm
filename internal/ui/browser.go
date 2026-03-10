package ui

import (
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type BrowserEntry struct {
	Name     string
	Path     string
	IsDir    bool
	Metadata string   // e.g. "Author | 15x15"
	Grid     [][]bool // true = black cell, false = white cell
}

// DrawBrowser displays a file-explorer style interface for selecting puzzles.
func DrawBrowser(screen tcell.Screen, title string, currentPath string, entries []BrowserEntry) int {
	selected := 0
	
	for {
		screen.Clear()
		w, h := screen.Size()

		// 1. Draw Header
		headerStyle := tcell.StyleDefault.Foreground(ColorTitle).Background(ColorBg).Bold(true)
		drawStr(screen, 2, 1, title, headerStyle)
		
		pathStyle := tcell.StyleDefault.Foreground(ColorSub).Background(ColorBg)
		drawStr(screen, 2, 2, "Path: "+currentPath, pathStyle)

		// 2. Define Box Area
		boxY := 4
		boxH := h - 7
		boxW := w - 4
		listW := (boxW * 2) / 3

		// Draw Borders
		borderStyle := tcell.StyleDefault.Foreground(tcell.ColorTeal).Background(ColorBg)
		
		// Horizontal lines
		for x := 2; x < 2+boxW; x++ {
			screen.SetContent(x, boxY, '─', nil, borderStyle)
			screen.SetContent(x, boxY+boxH, '─', nil, borderStyle)
		}
		// Vertical lines
		for y := boxY; y <= boxY+boxH; y++ {
			screen.SetContent(2, y, '│', nil, borderStyle)
			screen.SetContent(2+listW, y, '│', nil, borderStyle)
			screen.SetContent(2+boxW, y, '│', nil, borderStyle)
		}
		// Corners
		screen.SetContent(2, boxY, '┌', nil, borderStyle)
		screen.SetContent(2+boxW, boxY, '┐', nil, borderStyle)
		screen.SetContent(2, boxY+boxH, '└', nil, borderStyle)
		screen.SetContent(2+boxW, boxY+boxH, '┘', nil, borderStyle)
		screen.SetContent(2+listW, boxY, '┬', nil, borderStyle)
		screen.SetContent(2+listW, boxY+boxH, '┴', nil, borderStyle)

		// 3. Draw List
		maxLines := boxH - 1
		offset := 0
		if selected >= maxLines {
			offset = selected - maxLines/2
		}
		if len(entries)-offset < maxLines {
			offset = len(entries) - maxLines
		}
		if offset < 0 { offset = 0 }

		for i := 0; i < maxLines && i+offset < len(entries); i++ {
			idx := i + offset
			entry := entries[idx]
			
			style := tcell.StyleDefault.Foreground(ColorText).Background(ColorBg)
			prefix := "🧩 "
			if entry.IsDir {
				prefix = "📁 "
				style = style.Foreground(tcell.ColorYellow)
			}
			if entry.Name == "← Back" {
				prefix = ""
			}

			text := prefix + entry.Name
			text = runewidth.Truncate(text, listW-4, "...")
			
			finalX := 4
			finalY := boxY + 1 + i

			if idx == selected {
				style = tcell.StyleDefault.Foreground(ColorHighlight).Background(ColorHlBg)
				text = " " + text
				// Fill background for selection
				for x := 3; x < listW+2; x++ {
					screen.SetContent(x, finalY, ' ', nil, style)
				}
			}
			drawStr(screen, finalX, finalY, text, style)
		}

		// 4. Draw Metadata Pane (Right side)
		if len(entries) > 0 && selected >= 0 && selected < len(entries) {
			metaY := boxY + 2
			metaX := listW + 5
			curr := entries[selected]
			
			titleStyle := tcell.StyleDefault.Foreground(tcell.ColorTeal).Background(ColorBg).Bold(true)
			drawStr(screen, metaX, metaY, "DETAILS", titleStyle)
			
			if curr.IsDir {
				drawStr(screen, metaX, metaY+2, "Type: Directory", pathStyle)
			} else if curr.Name == "← Back" {
				drawStr(screen, metaX, metaY+2, "Return to previous menu", pathStyle)
			} else {
				// Show Metadata
				metaLines := strings.Split(curr.Metadata, "\n")
				metaStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(ColorBg)
				for j, mLine := range metaLines {
					drawStr(screen, metaX, metaY+2+j, mLine, metaStyle)
				}

				// Draw Grid Preview
				if curr.Grid != nil {
					gridY := metaY + 2 + len(metaLines) + 2
					
					// Ensure we don't draw outside the box
					for r, row := range curr.Grid {
						if gridY+r >= boxY+boxH { break }
						for c, isBlack := range row {
							if metaX+c*2 >= 2+boxW { break }
							
							char := ' '
							style := tcell.StyleDefault.Background(tcell.ColorWhite)
							if isBlack {
								style = tcell.StyleDefault.Background(tcell.ColorBlack)
							}
							
							// Using two chars for better aspect ratio in terminal
							screen.SetContent(metaX+c*2, gridY+r, char, nil, style)
							screen.SetContent(metaX+c*2+1, gridY+r, char, nil, style)
						}
					}
				}
			}
		}

		// 5. Help Footer
		footer := " [ENTER] Select/Enter  [ESC] Back  [↑/↓] Navigate "
		if runtime.GOOS != "darwin" {
		    // Show Alt shortcuts if applicable? No, navigation is just arrows here.
		}
		drawStr(screen, (w-runewidth.StringWidth(footer))/2, h-1, footer, pathStyle)

		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyUp {
				selected--
				if selected < 0 { selected = len(entries) - 1 }
			} else if ev.Key() == tcell.KeyDown {
				selected++
				if selected >= len(entries) { selected = 0 }
			} else if ev.Key() == tcell.KeyEnter {
				return selected
			} else if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return -1
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

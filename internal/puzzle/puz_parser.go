package puzzle

import (
	"encoding/binary"
	"errors"
	"os"
)

// ParsePuz reads a standard Across Lite .puz file and constructs a Puzzle struct.
func ParsePuz(filename string) (*Puzzle, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if len(data) < 0x34 {
		return nil, errors.New("file too short to be a valid .puz")
	}

	// Verify magic string "ACROSS&DOWN\x00"
	magic := string(data[0x02:0x0E])
	if magic != "ACROSS&DOWN\x00" {
		return nil, errors.New("invalid file magic string")
	}

	w := int(data[0x2C])
	h := int(data[0x2D])
	numClues := int(binary.LittleEndian.Uint16(data[0x2E:0x30]))

	gridSize := w * h
	if len(data) < 0x34+gridSize*2 {
		return nil, errors.New("corrupted .puz grid data")
	}

	boardStr := data[0x34 : 0x34+gridSize]
	// stateStr := data[0x34+gridSize : 0x34+gridSize*2]

	offset := 0x34 + gridSize*2
	readString := func() string {
		end := offset
		for end < len(data) && data[end] != 0 {
			end++
		}
		s := string(data[offset:end])
		offset = end + 1
		return s
	}

	title := readString()
	author := readString()
	copyright := readString()

	var clueTexts []string
	for i := 0; i < numClues; i++ {
		clueTexts = append(clueTexts, readString())
	}

	notes := ""
	if offset < len(data) {
		notes = readString()
	}

	hasSolution := false
	grid := NewGrid(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := boardStr[y*w+x]
			cell := &grid.Cells[y][x]
			if c == '.' {
				cell.IsBlack = true
			} else {
				cell.Solution = c
				if c != 'X' && c != '-' && c != ' ' {
					hasSolution = true
				}
			}
		}
	}

	// Calculate numbering and map clues
	var puzzleClues []Clue
	clueIndex := 0
	num := 1

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := grid.GetCell(x, y)
			if cell == nil || cell.IsBlack {
				continue
			}

			needsAcross := x == 0 || grid.Cells[y][x-1].IsBlack
			hasAcrossSpace := x+1 < w && !grid.Cells[y][x+1].IsBlack

			needsDown := y == 0 || grid.Cells[y-1][x].IsBlack
			hasDownSpace := y+1 < h && !grid.Cells[y+1][x].IsBlack

			isAcrossStart := needsAcross && hasAcrossSpace
			isDownStart := needsDown && hasDownSpace

			if isAcrossStart || isDownStart {
				cell.Number = num

				if isAcrossStart && clueIndex < len(clueTexts) {
					// Count length
					l := 0
					for tx := x; tx < w && !grid.Cells[y][tx].IsBlack; tx++ {
						l++
					}
					puzzleClues = append(puzzleClues, Clue{
						Number:    num,
						Direction: DirAcross,
						Text:      clueTexts[clueIndex],
						Length:    l,
						StartX:    x,
						StartY:    y,
					})
					clueIndex++
				}

				if isDownStart && clueIndex < len(clueTexts) {
					l := 0
					for ty := y; ty < h && !grid.Cells[ty][x].IsBlack; ty++ {
						l++
					}
					puzzleClues = append(puzzleClues, Clue{
						Number:    num,
						Direction: DirDown,
						Text:      clueTexts[clueIndex],
						Length:    l,
						StartX:    x,
						StartY:    y,
					})
					clueIndex++
				}
				num++
			}
		}
	}

	// Look for extra sections (like GEXT for circles/shading)
	for offset+8 < len(data) {
		sectionName := string(data[offset : offset+4])
		sectionLen := int(binary.LittleEndian.Uint16(data[offset+4 : offset+6]))
		// Skip header (name, len, checksum)
		sectionDataStart := offset + 8
		if sectionDataStart+sectionLen > len(data) {
			break
		}
		sectionData := data[sectionDataStart : sectionDataStart+sectionLen]

		if sectionName == "GEXT" {
			for i := 0; i < len(sectionData) && i < gridSize; i++ {
				y := i / w
				x := i % w
				mask := sectionData[i]
				grid.Cells[y][x].IsCircled = (mask & 0x40) != 0
				grid.Cells[y][x].IsShaded = (mask & 0x80) != 0
			}
		}

		// Move to next section: name(4) + len(2) + checksum(2) + data(L) + null(1)
		offset += 8 + sectionLen + 1
	}

	p := &Puzzle{
		Title:       title,
		Author:      author,
		Copyright:   copyright,
		Notes:       notes,
		Grid:        grid,
		Clues:       puzzleClues,
		HasSolution: hasSolution,
	}

	return p, nil
}

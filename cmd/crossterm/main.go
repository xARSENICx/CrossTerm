package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"crossterm/internal/engine"
	"crossterm/internal/modes"
	"crossterm/internal/netproto"
	"crossterm/internal/puzzle"
	inputsystem "crossterm/internal/systems/input"
	networksystem "crossterm/internal/systems/network"
	puzzlesystem "crossterm/internal/systems/puzzle"
	rendersystem "crossterm/internal/systems/render"
	savesystem "crossterm/internal/systems/save"
	"crossterm/internal/ui"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type InviteData struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

func createDemoPuzzle() *puzzle.Puzzle {
	grid := puzzle.NewGrid(5, 5)

	board := []string{
		"CAT.D",
		"O.RUN",
		"LOG.A",
		"D.USE",
		"SAP..",
	}

	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			c := board[y][x]
			if c == '.' {
				grid.Cells[y][x].IsBlack = true
			} else {
				grid.Cells[y][x].Solution = c
			}
		}
	}

	clues := []puzzle.Clue{
		{Number: 1, Direction: puzzle.DirAcross, Text: "Feline pet", Length: 3, StartX: 0, StartY: 0},
		{Number: 2, Direction: puzzle.DirDown, Text: "Winter feeling", Length: 5, StartX: 0, StartY: 0},
	}

	grid.Cells[0][0].Number = 1
	grid.Cells[0][0].Number = 2

	return &puzzle.Puzzle{
		Title:     "Demo Puzzle",
		Author:    "Cryptic Engine",
		Copyright: "2026",
		Grid:      grid,
		Clues:     clues,
	}
}

func main() {
	puzFile := flag.String("file", "", "Path to .puz file")
	flag.Parse()

	// Initialize terminal screen early for interactive UI
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Failed to create screen: %v", err)
	}

	if err := screen.Init(); err != nil {
		log.Fatalf("Failed to init screen: %v", err)
	}
	defer screen.Fini()

	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))

	var p *puzzle.Puzzle

	if *puzFile != "" {
		p, err = puzzle.ParsePuz(*puzFile)
		if err != nil {
			log.Fatalf("Failed to parse puzzle: %v", err)
		}
		// If loaded directly from CLI args, skip menus and launch right into Solo
		playGame(screen, p, "solo", "not_timed_standard", false, nil, nil, "")
		return
	}

	for {
		// 1. Top Level Menu
		topChoice := ui.DrawMenu(screen, "CrossTerm : Crosswords right into your terminal\n\nWhat do your plans look like today?", []ui.MenuOption{
			{Text: "Choose Game Mode", Val: "play"},
			{Text: "Load Puzzle from Aggregators", Val: "download"},
			{Text: "See Puzzle Directory", Val: "library"},
			{Text: "Game Controls Guide", Val: "controls"},
			{Text: "Exit", Val: "exit"},
		})

		if topChoice == -1 || (topChoice == 4) {
			return // Escaped or Exit
		}

		switch topChoice {
		case 3:
			ui.DrawControls(screen)
			continue
		case 0:
		play_flow:
			// 2. Play Flow -> Solo / Duel / coop
			modeChoice := ui.DrawMenu(screen, "Game Mode\n\nChoose your destiny:", []ui.MenuOption{
				{Text: "Solo Mode", Val: "solo"},
				{Text: "Duel Mode", Val: "duel"},
				{Text: "Co-operative", Val: "coop"},
				{Text: "Back ←", Val: "back"},
			})
			if modeChoice == -1 || modeChoice == 3 {
				continue
			} // go to top menu

			if modeChoice == 2 {
				ui.DrawText(screen, "Co-operative Mode\n\nP2P Solving Together is under architecture.\nComing soon to a terminal near you!", true)
				goto play_flow
			}

		rules_flow:
			gameMode := ""
			subMode := ""

			if modeChoice == 0 {
			timing_flow:
				timingChoice := ui.DrawMenu(screen, "Solo Mode\nSelect Timing Rules", []ui.MenuOption{
					{Text: "Not timed", Val: "not_timed"},
					{Text: "Timed", Val: "timed"},
					{Text: "← Back", Val: "back"},
				})
				if timingChoice == -1 || timingChoice == 2 {
					goto play_flow
				}

				timingPrefix := []string{"not_timed", "timed"}[timingChoice]

				featureChoice := ui.DrawMenu(screen, "Solo Mode\nSelect Features", []ui.MenuOption{
					{Text: "Standard (No Assistance)", Val: "standard"},
					{Text: "With assistance (checks enabled)", Val: "checks"},
					{Text: "With anagrammer", Val: "tools"},
					{Text: "← Back", Val: "back"},
				})
				if featureChoice == -1 || featureChoice == 3 {
					goto timing_flow
				}

				featureSuffix := []string{"standard", "checks", "tools"}[featureChoice]

				gameMode = "solo"
				subMode = timingPrefix + "_" + featureSuffix
			} else {
				rulesChoice := ui.DrawMenu(screen, "Duel Mode Rules\nSelect Ruleset", []ui.MenuOption{
					{Text: "Blind Duel (+10s per error)", Val: "blind"},
					{Text: "Race Duel (Live scores)", Val: "race"},
					{Text: "Race Duel with Checks", Val: "race_chk"},
					{Text: "Race Duel with Tools", Val: "race_tools"},
					{Text: "← Back", Val: "back"},
				})
				if rulesChoice == -1 || rulesChoice == 4 {
					goto play_flow
				}
				gameMode = "duel"
				subMode = []string{"blind", "race", "race_chk", "race_tools"}[rulesChoice]
			}

			var conn *net.UDPConn
			var peerAddr *net.UDPAddr
			var roomID string
			isHost := false

			if gameMode == "duel" {
			role_flow:
				roleChoice := ui.DrawMenu(screen, "Duel Connection setup\n\nSelect Role", []ui.MenuOption{
					{Text: "Host a Duel", Val: "host"},
					{Text: "Join a Duel", Val: "join"},
					{Text: "← Back", Val: "back"},
				})
				if roleChoice == -1 || roleChoice == 2 {
					goto play_flow
				}
				isHost = roleChoice == 0

				if isHost {
					p = selectPuzzle(screen)
					if p == nil {
						goto role_flow
					}
				} else {
					p = nil // Joiner waits for puzzle from host
				}

				conn, peerAddr, roomID = setupNetwork(screen, isHost)
				if conn == nil {
					goto role_flow
				}
			} else {
				// Solo Mode
				p = selectPuzzle(screen)
				if p == nil {
					goto rules_flow
				}
			}

			// 5. Start Game
			playGame(screen, p, gameMode, subMode, isHost, conn, peerAddr, roomID)
			return // Ensure we exit when the game is done

		case 1:
			ui.DrawText(screen, "Aggregator fetching is not implemented yet.\n\nPress any key to return to menu.", true)

		case 2:
			libPuz := selectPuzzle(screen)
			if libPuz != nil {
				ui.DrawText(screen, fmt.Sprintf("Puzzle: %s\nAuthor: %s\nSize: %dx%d\n\n(Feature coming soon: view details without playing)\n\nPress any key to return.", libPuz.Title, libPuz.Author, libPuz.Grid.Width, libPuz.Grid.Height), true)
			}
		}
	}
}

// selectPuzzle displays a navigable file explorer for raw/local puzzles and returns the parsed Puzzle.
func selectPuzzle(screen tcell.Screen) *puzzle.Puzzle {
	currentDir := "data/puzzles"
	for {
		var entries []ui.BrowserEntry
		
		// 1. Scan directory for sub-folders and puzzle files
		items, err := os.ReadDir(currentDir)
		if err == nil {
			// Directories first for easy navigation
			for _, item := range items {
				if item.IsDir() && !strings.HasPrefix(item.Name(), ".") {
					entries = append(entries, ui.BrowserEntry{
						Name:  item.Name(),
						Path:  filepath.Join(currentDir, item.Name()),
						IsDir: true,
					})
				}
			}
			// Then .puz files
			for _, item := range items {
				if !item.IsDir() && strings.HasSuffix(strings.ToLower(item.Name()), ".puz") {
					path := filepath.Join(currentDir, item.Name())
					meta := ""
					var gridPreview [][]bool
					if parsed, pErr := puzzle.ParsePuz(path); pErr == nil {
						auth := parsed.Author
						if auth == "" { auth = "Unknown" }
						
						maxDim := parsed.Grid.Width
						if parsed.Grid.Height > maxDim { maxDim = parsed.Grid.Height }
						
						pType := "Standard"
						switch {
						case maxDim <= 7: pType = "Mini"
						case maxDim <= 13: pType = "Midi"
						case maxDim > 18: pType = "Sunday/Jumbo"
						}
						
						meta = fmt.Sprintf("Title: %s\nAuthor: %s\nSize: %dx%d (%s)", parsed.Title, auth, parsed.Grid.Width, parsed.Grid.Height, pType)
						
						// Create simple boolean grid for preview
						gridPreview = make([][]bool, parsed.Grid.Height)
						for y := 0; y < parsed.Grid.Height; y++ {
							gridPreview[y] = make([]bool, parsed.Grid.Width)
							for x := 0; x < parsed.Grid.Width; x++ {
								gridPreview[y][x] = parsed.Grid.Cells[y][x].IsBlack
							}
						}
					} else {
						meta = "Error parsing .puz file"
					}
					entries = append(entries, ui.BrowserEntry{
						Name:     item.Name(),
						Path:     path,
						IsDir:    false,
						Metadata: meta,
						Grid:     gridPreview,
					})
				}
			}
		}

		// System options
		entries = append(entries, ui.BrowserEntry{Name: "Built-in Demo Puzzle", Path: "demo"})
		entries = append(entries, ui.BrowserEntry{Name: "← Back", Path: "back"})

		// 2. Draw the immersive browser UI
		choice := ui.DrawBrowser(screen, " CROSS-TERM : PUZZLE EXPLORER ", currentDir, entries)
		if choice == -1 {
			return nil
		}
		
		selected := entries[choice]
		
		if selected.Path == "back" {
			if currentDir == "data/puzzles" {
				return nil
			}
			currentDir = filepath.Dir(currentDir)
			continue
		}
		
		if selected.Path == "demo" {
			return createDemoPuzzle()
		}
		
		if selected.IsDir {
			currentDir = selected.Path
			continue
		}

		// 3. Selection is a file
		p, pErr := puzzle.ParsePuz(selected.Path)
		if pErr != nil {
			ui.DrawText(screen, "Error loading puzzle:\n"+pErr.Error(), true)
			continue
		}
		return p
	}
}

// setupNetwork handles Host/Join handshake via the UDP relay server.
func setupNetwork(screen tcell.Screen, isHost bool) (*net.UDPConn, *net.UDPAddr, string) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		panic(err)
	}

	relayAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9000") // Replace with remote VPS IP in production
	var peerAddr *net.UDPAddr
	var roomID string

	if isHost {
		roomID = ui.DrawInput(screen, ">>> YOU ARE HOSTING <<<", "Create a 4-Character Room ID (e.g. ABCD):")
		if roomID == "" {
			return nil, nil, ""
		}

		// Send Create Room
		msg := netproto.NetworkMessage{Type: netproto.MsgCreateRoom, RoomID: roomID}
		bMsg, _ := json.Marshal(msg)
		conn.WriteToUDP(bMsg, relayAddr)

		ui.DrawText(screen, fmt.Sprintf("Room [%s] Created!\n\nWaiting for Joiner to connect to Relay...", roomID), false)

		// Wait for Match
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			panic("Timeout waiting for joiner")
		}
		var resp netproto.NetworkMessage
		json.Unmarshal(buffer[:n], &resp)
		peerAddr, _ = net.ResolveUDPAddr("udp", resp.PeerIP)

		ui.DrawText(screen, "\nStarting Hybrid P2P & Engine...", false)
	} else {
		roomID = ui.DrawInput(screen, ">>> YOU ARE JOINING <<<", "Enter the Host's 4-Character Room ID:")
		if roomID == "" {
			return nil, nil, ""
		}

		// Send Join Room
		msg := netproto.NetworkMessage{Type: netproto.MsgJoinRoom, RoomID: roomID}
		bMsg, _ := json.Marshal(msg)
		conn.WriteToUDP(bMsg, relayAddr)

		ui.DrawText(screen, fmt.Sprintf("Joining Room [%s] via Relay...\n\nWaiting for Host to launch Game...", roomID), false)

		// Wait for Match
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			panic("Failed to find room or timed out")
		}
		var resp netproto.NetworkMessage
		json.Unmarshal(buffer[:n], &resp)
		peerAddr, _ = net.ResolveUDPAddr("udp", resp.PeerIP)
	}

	conn.SetReadDeadline(time.Time{}) // reset deadline
	return conn, peerAddr, roomID
}

func playGame(screen tcell.Screen, p *puzzle.Puzzle, gameMode string, subMode string, isHost bool, conn *net.UDPConn, peerAddr *net.UDPAddr, roomID string) {
	screen.Clear()
	screen.EnableMouse()
	screen.Show()

	eb := engine.NewEventBus()
	coreEngine := engine.NewCoreEngine(eb, p)
	coreEngine.State.Mode = subMode
	coreEngine.State.IsDuel = (conn != nil)
	coreEngine.SetMode(modes.GetMode(subMode))

	if conn != nil && peerAddr != nil {
		netSys := networksystem.NewNetworkSystem(eb, coreEngine.State, conn, peerAddr, isHost)
		netSys.SetRelayFallback("127.0.0.1:9000", roomID)
		go netSys.Run()
	}

	inSys := inputsystem.NewInputSystem(screen, eb)
	puzSys := puzzlesystem.NewPuzzleSystem(eb, coreEngine.State)
	renSys := rendersystem.NewRenderSystem(screen, eb, coreEngine.State)
	saveSys := savesystem.NewSaveSystem(eb, coreEngine.State)

	if !isHost && gameMode == "solo" {
		saveSys.Load()
	}

	go inSys.Run()
	go puzSys.Run()
	go renSys.Run()
	go saveSys.Run()

	eb.Publish(engine.Event{Type: engine.EventStateUpdate})
	coreEngine.Run()
}

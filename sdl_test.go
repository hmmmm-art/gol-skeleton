package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/sdl"
)

var paramRequests chan gol.Params
var testsComplete chan bool
var sdlEvents chan gol.Event
var sdlAlive chan int

func runSdl(p gol.Params, noVis *bool) {
	var w *sdl.Window = nil
	if !(*noVis) {
		w = sdl.NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
	}

	board := make([][]byte, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		board[i] = make([]byte, p.ImageWidth)
	}

sdlLoop:
	for {
		if w != nil {
			w.PollEvent()
		}

		select {
		case event, ok := <-sdlEvents:
			if !ok {
				if w != nil {
					w.Destroy()
				}
				break sdlLoop
			}

			switch e := event.(type) {
			case gol.CellFlipped:
				board[e.Cell.Y][e.Cell.X] = ^board[e.Cell.Y][e.Cell.X]
				if w != nil {
					w.FlipPixel(e.Cell.X, e.Cell.Y)
				}

			case gol.TurnComplete:
				if w != nil {
					w.RenderFrame()
				}
				count := 0
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						if board[y][x] == 255 {
							count++
						}
					}
				}
				sdlAlive <- count

			case gol.FinalTurnComplete:
				if w != nil {
					w.Destroy()
				}
				break sdlLoop

			default:
				if len(event.String()) > 0 {
					fmt.Printf("Completed Turns %-8v%v\n", event.GetCompletedTurns(), event)
				}
			}
		default:
			break
		}
	}
}

func TestMain(m *testing.M) {
	runtime.LockOSThread()
	noVis := flag.Bool("noVis", false,
		"Disables the SDL window, so there is no visualisation during the tests.")
	flag.Parse()

	// p := gol.Params{ImageWidth: 512, ImageHeight: 512}

	paramRequests = make(chan gol.Params)
	testsComplete = make(chan bool)

	sdlEvents = make(chan gol.Event)
	sdlAlive = make(chan int)
	result := make(chan int)

	go func() {
		res := m.Run()
		// go func() {
		// 	sdlEvents <- gol.FinalTurnComplete{}
		// }()
		testsComplete <- true
		result <- res
	}()

	running := true
	for running {
		select {
		case p := <-paramRequests:
			runSdl(p, noVis)
		case <-testsComplete:
			running = false
		}
	}

	os.Exit(<-result)
}

func sdlFail(t *testing.T, message string) {
	t.Log(message)
	time.Sleep(5 * time.Second)
	sdlEvents <- gol.FinalTurnComplete{}
	t.FailNow()
}

// TestSdl tests a 512x512 image for 100 turns using 8 worker threads.
func TestSdl(t *testing.T) {
	p := gol.Params{ImageWidth: 512, ImageHeight: 512, Turns: 100, Threads: 8}
	paramRequests <- p

	testName := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
	alive := readAliveCounts(p.ImageWidth, p.ImageHeight)
	t.Run(testName, func(t *testing.T) {
		turnNum := 0
		events := make(chan gol.Event)
		go gol.Run(p, events, nil)
		time.Sleep(2 * time.Second)
		final := false
		for event := range events {
			switch e := event.(type) {
			case gol.CellFlipped:
				sdlEvents <- e
			case gol.TurnComplete:
				turnNum++

				if turnNum != e.CompletedTurns {
					sdlFail(t, fmt.Sprintf("Incorrect turn number for TurnComplete. Was %d, should be %d.", e.CompletedTurns, turnNum))
				}

				if e.CompletedTurns > p.Turns {
					sdlFail(t, fmt.Sprintf("Too many TurnComplete events sent. Last TurnComplete was for turn %d. Simulation should only run for %d turns.", e.CompletedTurns, p.Turns))
				}

				sdlEvents <- e
				aliveCount := <-sdlAlive
				if alive[turnNum] != aliveCount {
					sdlFail(t, fmt.Sprintf("Incorrect number of alive cells displayed on turn %d. Was %d, should be %d.", turnNum, aliveCount, alive[turnNum]))
				}
			case gol.FinalTurnComplete:
				if e.CompletedTurns != p.Turns {
					sdlFail(t, fmt.Sprintf("Incorrect final turn number. Was %d, should be %d.", e.CompletedTurns, p.Turns))
				}

				if turnNum < p.Turns {
					sdlFail(t, fmt.Sprintf("More TurnComplete events expected before FinalTurnComplete. Last TurnComplete was for turn %d.", turnNum))
				}

				final = true
				sdlEvents <- e
			}
		}

		if !final {
			sdlEvents <- gol.FinalTurnComplete{}
			t.Fatal("Simulation finished without sending a FinalTurnComplete event.")
		}
	})
}

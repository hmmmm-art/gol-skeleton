package gol

import (
	"fmt"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// Returns a function (like a getter) allowing us to access data without risk of overwriting
func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

// distributor divides the work between workers and interacts with other goroutines.

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}

	return aliveCells
}

func distributor(p Params, c distributorChannels) {

    c.events <- StateChange{CompletedTurns: 0, NewState: Executing}

	// Create initial world
	world := make([][]byte, p.ImageHeight)
	for col := range world {
		world[col] = make([]byte, p.ImageWidth)
	}

	// Setup input
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// Create channels for workers
	resultChannels := make([]chan [][]uint8, p.Threads)
	for v := range resultChannels {
		resultChannels[v] = make(chan [][]uint8)
	}

	// Read initial state
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			if cell == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
			world[y][x] = cell
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// Calculate slice height for each worker
	sliceHeight := p.ImageHeight / p.Threads
	remainder := p.ImageHeight % p.Threads

	//Create a ticker that will send signal to ticker.C channel every 2 seconds
    ticker := time.NewTicker(2 * time.Second)
    //Ticker will stop when distributor functions ends
	defer ticker.Stop()

	// Create a channel for ticker done
	tickerDone := make(chan bool)
    //tickerDone will close when distributor functions ends
	defer close(tickerDone)

	// Start a separate goroutine for the ticker
	go func() {
		for {
			select {
			case <-ticker.C:
				c.events <- AliveCellsCount{turn, len(calculateAliveCells(p, world))}
			case <-tickerDone:
				return
			}
		}
	}()

	// Main game loop
	for turn = 0; turn < p.Turns; turn++ {
		newWorld := make([][]byte, p.ImageHeight)
		for i := range newWorld {
			newWorld[i] = make([]byte, p.ImageWidth)
		}

		// Launch workers
		for i := 0; i < p.Threads; i++ {
			startRow := i * sliceHeight
			endRow := (i+1)*sliceHeight - 1
			if i == p.Threads-1 {
				endRow += remainder
			}
			go worker(startRow, endRow, world, c.events, resultChannels[i], p, turn)
		}

		// Collect results from all workers
		for i := 0; i < p.Threads; i++ {
			workerResult := <-resultChannels[i]
			// Copy worker results to the new world
			startRow := i * sliceHeight
			for y := 0; y < len(workerResult); y++ {
				for x := 0; x < p.ImageWidth; x++ {
					newWorld[startRow+y][x] = workerResult[y][x]
				}
			}
		}

		world = newWorld
		c.events <- TurnComplete{CompletedTurns: turn}
	}

	// Signal ticker goroutine to stop
	tickerDone <- true

	// Calculate final alive cells
	aliveCells := calculateAliveCells(p, world)
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	// Output the final state as a PGM image
	outputFilename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, p.Turns)
	c.ioCommand <- ioOutput
	c.ioFilename <- outputFilename

	// Stream each cell's value to ioOutput channel to write the final world state.
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.events <- ImageOutputComplete{CompletedTurns: p.Turns, Filename: outputFilename}

	// Cleanup
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}
	close(c.events)
}

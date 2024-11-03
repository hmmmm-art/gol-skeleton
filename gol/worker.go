package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

func worker(startRow, endRow int, world [][]byte, events chan<- Event, resultChannel chan<- [][]uint8, p Params, turn int) {
    chunkHeight := (endRow - startRow) + 1
    newChunk := make([][]uint8, chunkHeight)
    for i := range newChunk {
        newChunk[i] = make([]uint8, p.ImageWidth)
    }

    for y := startRow; y <= endRow; y++ {
        for x := 0; x < p.ImageWidth; x++ {
            aliveNeighbors := 0
            for dy := -1; dy <= 1; dy++ {
                for dx := -1; dx <= 1; dx++ {
                    if dx == 0 && dy == 0 {
                        continue
                    }
                    neighborY := (y + dy + p.ImageHeight) % p.ImageHeight
                    neighborX := (x + dx + p.ImageWidth) % p.ImageWidth
                    if world[neighborY][neighborX] == 255 {
                        aliveNeighbors++
                    }
                }
            }

            cellValue := world[y][x]
            newValue := cellValue
            
            switch {
            case cellValue == 255 && (aliveNeighbors < 2 || aliveNeighbors > 3):
                newValue = 0
                events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
            case cellValue == 0 && aliveNeighbors == 3:
                newValue = 255
                events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
            }
            
            newChunk[y-startRow][x] = newValue
        }
    }

    resultChannel <- newChunk
}
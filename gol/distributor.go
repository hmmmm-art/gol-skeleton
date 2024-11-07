package gol


import (
   "fmt"
   "net/rpc"
   "time"
   "os"
   "uk.ac.bris.cs/gameoflife/stubs"
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


func countAliveNeighbors(x, y int, world [][]byte, p Params) int {
   alive := 0
   height := p.ImageHeight
   width := p.ImageWidth


   // Loop through the 3x3 grid surrounding the cell
   for deltaY := -1; deltaY <= 1; deltaY++ {
       for deltaX := -1; deltaX <= 1; deltaX++ {
           // Skip the cell itself
           if deltaY == 0 && deltaX == 0 {
               continue
           }
           neighborX := (x + deltaX + width) % width
           neighborY := (y + deltaY + height) % height
           if world[neighborY][neighborX] == 255 {
               alive++
           }
       }
   }
   return alive
}


func runGameOfLife(p Params, world [][]byte, c distributorChannels) [][]byte {
   for turn := 0; turn < p.Turns; turn++ {
       newWorld := make([][]byte, p.ImageHeight)
       for y := range world {
           newWorld[y] = make([]byte, p.ImageWidth)
           for x := range world[y] {
               aliveNeighbors := countAliveNeighbors(x, y, world, p)
               cellValue := world[y][x]
               newCellValue := cellValue


               // Apply Game of Life rules
               if cellValue == 255 {
                   if aliveNeighbors < 2 || aliveNeighbors > 3 {
                       newCellValue = 0
                       c.events <- CellFlipped{CompletedTurns: turn + 1, Cell: util.Cell{X: x, Y: y}}
                   }
               } else {
                   if aliveNeighbors == 3 {
                       newCellValue = 255
                       c.events <- CellFlipped{CompletedTurns: turn + 1, Cell: util.Cell{X: x, Y: y}}
                   }
               }
               newWorld[y][x] = newCellValue
           }
       }
       world = newWorld
       c.events <- TurnComplete{CompletedTurns: turn + 1}
   }
   return world
}




func screenShot(world [][]uint8, turn int, p Params, c distributorChannels) {
   filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
   c.ioCommand <- ioOutput
   c.ioFilename <- filename
   for y := 0; y < p.ImageHeight; y++ {
       for x := 0; x < p.ImageWidth; x++ {
           c.ioOutput <- world[y][x]
       }
   }
   c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}


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


func handleServerResponse(res *stubs.Response, ticker *time.Ticker, p Params, c distributorChannels) {
   ticker.Stop()
   c.events <- FinalTurnComplete{
       CompletedTurns: res.CurrentTurn,
       Alive:          calculateAliveCells(p, res.State),
   }
   c.events <- StateChange{
       CompletedTurns: res.CurrentTurn,
       NewState:       Quitting,
   }
   screenShot(res.State, res.CurrentTurn, p, c)
}


func handleKeyPress(key rune, client *rpc.Client, ticker *time.Ticker, p Params, c distributorChannels) bool {
   request := new(stubs.Request)
   response := new(stubs.Response)


   switch key {
   case 'q':
       return handleQuit(client, ticker, request, response, p, c)


   case 's':
       client.Call(stubs.ScreenShot, request, response)
       screenShot(response.State, response.CurrentTurn, p, c)


   case 'p':
       handlePause(client, request, response, c)


   case 'k':
       return handleKill(client, ticker, request, response, p, c)
   }


   return false
}


func handleQuit(client *rpc.Client, ticker *time.Ticker, req *stubs.Request, res *stubs.Response, p Params, c distributorChannels) bool {
   ticker.Stop()
   client.Call(stubs.QuitGame, req, res)
   screenShot(res.State, res.CurrentTurn, p, c)
   c.events <- StateChange{
       CompletedTurns: res.CurrentTurn,
       NewState:       Quitting,
   }
   return true
}


func handlePause(client *rpc.Client, req *stubs.Request, res *stubs.Response, c distributorChannels) {
   client.Call(stubs.PauseGame, req, res)
   newState := Executing
   if res.Paused {
       newState = Paused
   }
   c.events <- StateChange{
       CompletedTurns: res.CurrentTurn,
       NewState:       newState,
   }
   if !res.Paused {
       fmt.Println("Continuing")
   }
}


func handleKill(client *rpc.Client, ticker *time.Ticker, req *stubs.Request, res *stubs.Response, p Params, c distributorChannels) bool {
   ticker.Stop()
   client.Call(stubs.KillGame, req, res)
   client.Close()
   screenShot(res.State, res.CurrentTurn, p, c)
   c.events <- StateChange{
       CompletedTurns: res.CurrentTurn,
       NewState:       Quitting,
   }
   return true
}


// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
   // Create a 2D slice to store the world
   world := make([][]byte, p.ImageHeight)
   for y := range world {
       world[y] = make([]byte, p.ImageWidth)
   }


   // Read the initial state from the input file
   filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
   c.ioCommand <- ioInput
   c.ioFilename <- filename
   for y := 0; y < p.ImageHeight; y++ {
       for x := 0; x < p.ImageWidth; x++ {
           cell := <-c.ioInput
           if cell == 255 {
               c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
           }
           world[y][x] = cell
       }
   }


   // Attempt to connect to the broker
   client, err := rpc.Dial("tcp", "127.0.0.1:8030")
   if err != nil || os.Getenv("BROKER_DISABLED") == "true" {
       fmt.Println("Broker not available or running in test mode, executing locally.", err)


       // Run the Game of Life locally
       world = runGameOfLife(p, world, c)
   } else {
       defer client.Close()
       // Start Game of Life on the server
       request := stubs.Request{CurrentState: world, Params: stubs.Params(p)}
       response := new(stubs.Response)
       err = client.Call(stubs.GolEngine, request, response)
       if err != nil {
           fmt.Println("Error calling GolEngine on broker:", err)
           return
       }


       world = response.State
       // Send TurnComplete events
       for turn := 1; turn <= p.Turns; turn++ {
           c.events <- TurnComplete{CompletedTurns: turn}
       }
   }


   // Finalize and output the results
   c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}
   c.events <- StateChange{CompletedTurns: p.Turns, NewState: Quitting}
   screenShot(world, p.Turns, p, c)
   c.ioCommand <- ioCheckIdle
   <-c.ioIdle
   close(c.events)
}


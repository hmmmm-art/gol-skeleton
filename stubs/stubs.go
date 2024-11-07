package stubs


import (


   "uk.ac.bris.cs/gameoflife/util"
)


var BrokerConnect = "Broker.ConnectNode"
var GolEngine = "Broker.GolEngine"
var CalculateAliveCells = "Broker.CalculateAliveCells"
var QuitGame = "Broker.QuitGame"
var KillGame = "Broker.KillGame"
var PauseGame = "Broker.PauseGame"
var ScreenShot = "Broker.ScreenShot"
var KillWorker = "Worker.KillGame"  
var WorkerGolEngine = "Worker.GolEngine"      // Method to kill the game


type Response struct {
   State [][]uint8
   CurrentTurn int
   AliveCount int
   Paused bool
}


type Status struct {
   Success bool   // True if the operation was successful; false otherwise
   Error   string // Error message if Success is false
}


type Request struct {
   CurrentState [][]uint8 // The current state of the grid (2D slice of cells)
   Params       Params    // Game configuration parameters (grid size, turns, etc.)
}


type Params struct {
   Turns       int
   Threads     int
   ImageWidth  int
   ImageHeight int
}


type ConnectNode struct {
   IpAddress string
   Function string
}


// NodeArgs holds the data each worker needs to process a part of the game.
type NodeArgs struct {
   WorldSlice [][]uint8 // Portion of the grid that the worker processes
   YStart     int       // Starting row index for this worker
   YEnd       int       // Ending row index for this worker
   ImageWidth int   // Game parameters (width, height, etc.)


}
type NodeResp struct {
   WorldSlice [][]uint8
}


// Subscribe holds information needed to register a worker node with the broker
type Subscribe struct {
   IpAddress string // IP address of the worker node
}


// AliveCellsResponse holds data about the alive cells in the grid
type AliveCellsResponse struct {
   AliveCells []util.Cell // List of alive cell coordinates
   AliveCount int         // Total number of alive cells
   Turn int
}


type CellCount struct {
   Turn       int
   CellsCount int
}






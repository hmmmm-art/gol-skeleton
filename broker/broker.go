package main


import (
   "flag"
   "fmt"
   "net"
   "net/rpc"
   "sync"


   "uk.ac.bris.cs/gameoflife/stubs"
)


type JobList struct {
   Args       stubs.NodeArgs
   ResultChan chan [][]uint8
}
type Broker struct {
   world      [][]uint8
   turn       int
   pause      bool
   quit       bool
   listener   net.Listener
   jobs       []chan JobList
   killSignal chan bool
   lock       sync.Mutex
   wait       sync.WaitGroup
   clients    []*rpc.Client
}


func countAliveCells(p stubs.Params, world [][]byte) int {
   count := 0
   for _, row := range world {
       for _, cellValue := range row {
           if cellValue == 255 {
               count++
           }
       }
   }
   return count
}


func (br *Broker) Subscribe(request stubs.Subscribe, response *stubs.Status) (err error) {
   br.wait.Add(1)
   defer br.wait.Done()


   // Connect a new worker node
   client, err := rpc.Dial("tcp", request.IpAddress)
   if err != nil {
       response.Success = false
       response.Error = err.Error()
       return err
   }


   br.lock.Lock()
   br.clients = append(br.clients, client)
   br.lock.Unlock()


   response.Success = true
   response.Error = ""


   return nil
}


func (br *Broker) GolEngine(request stubs.Request, response *stubs.Response) error {


   fmt.Println(br.clients)
   world := request.CurrentState
   sliceHeight := request.Params.ImageHeight / len(br.clients)
   remainder := request.Params.ImageHeight % len(br.clients)
   turn := 0


   returnChans := make([]chan [][]byte, len(br.clients))
   for i := range returnChans {
       returnChans[i] = make(chan [][]byte)
   }


   for turn = 0; turn < request.Params.Turns; {


       fmt.Println("CALCULATING")
       for i := 0; i < len(br.clients); i++ {


           startRow := i * sliceHeight
           endRow := (i+1)*sliceHeight - 1
           if i == len(br.clients)-1 {
               endRow += remainder
           }


           nodeReq := stubs.NodeArgs{
               WorldSlice: world,
               YStart:     startRow,
               YEnd:       endRow,
               ImageWidth: request.Params.ImageWidth,
           }
           nodeResp := new(stubs.NodeResp)
           br.wait.Add(1)
           // Call the GolEngine method on the first client in the br.clients slice
           go func()  {
               defer br.wait.Done()
               err := br.clients[i].Call(stubs.WorkerGolEngine, nodeReq, nodeResp)
               if err != nil {
                   response.State = nil
                   response.CurrentTurn = 0
               }
               returnChans[i] <- nodeResp.WorldSlice
           }()
       }


       for _, ch := range returnChans {
           world = append(world, <-ch...)
       }
       turn++
       // Update the broker's state
       br.lock.Lock()
       br.world = world
       br.turn = turn
       br.lock.Unlock()
   }


   fmt.Println("FINISHED")
   br.lock.Lock()
   response.State = world
   response.CurrentTurn = turn
   br.world = nil
   br.turn = 0
   br.lock.Unlock()
   return nil
}
func (br *Broker) CalculateAliveCells(request stubs.Request, response *stubs.AliveCellsResponse) (err error) {
   br.wait.Add(1)
   defer br.wait.Done()


   count := countAliveCells(request.Params, br.world)
   br.lock.Lock()
   response.Turn = br.turn
   response.AliveCount = count
   br.lock.Unlock()


   return
}


func (br *Broker) QuitGame(request stubs.Request, response *stubs.Response) (err error) {
   br.wait.Add(1)
   defer br.wait.Done()


   br.lock.Lock()
   response.State = br.world
   br.quit = true
   br.pause = false
   response.CurrentTurn = br.turn
   br.lock.Unlock()


   return
}


func (br *Broker) KillGame(request stubs.Request, response *stubs.Response) (err error) {


   br.wait.Add(1)
   defer br.wait.Done()


   for _, client := range br.clients {
       req := new(stubs.Request)
       response := new(stubs.Response)
       client.Call(stubs.KillWorker, req, response)
   }


   br.lock.Lock()
   response.State = br.world
   response.CurrentTurn = br.turn
   br.quit = true
   br.lock.Unlock()


   br.killSignal <- true


   return


}


func (br *Broker) PauseGame(request stubs.Request, response *stubs.Response) (err error) {


   br.wait.Add(1)
   defer br.wait.Done()


   br.lock.Lock()
   br.pause = !br.pause
   response.CurrentTurn = br.turn
   response.Paused = br.pause
   br.lock.Unlock()


   return


}


func (br *Broker) ScreenShot(request stubs.Request, response *stubs.Response) (err error) {


   br.wait.Add(1)
   defer br.wait.Done()


   br.lock.Lock()
   response.State = br.world
   response.CurrentTurn = br.turn
   br.lock.Unlock()


   return
}


func main() {
   pAddr := flag.String("port", ":8030", "to listen to")
   flag.Parse()
   listener, err := net.Listen("tcp", *pAddr)
   if err != nil {
       fmt.Println(err)
   }
   br := &Broker{}
   rpc.Register(br)
   br.listener = listener
   br.jobs = make([]chan JobList, 16) // Initialize the slice with 16 elements
   for i := range br.jobs {
       br.jobs[i] = make(chan JobList) // Initialize each element as a channel of type JobList
   }
   br.killSignal = make(chan bool)
   defer br.listener.Close()
   go rpc.Accept(br.listener)
   <-br.killSignal
}




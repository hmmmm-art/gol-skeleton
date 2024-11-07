package main


import (
   "flag"
   "fmt"
   "net"
   "net/rpc"
   "sync"


   "uk.ac.bris.cs/gameoflife/stubs"
)


func (*Worker) GolEngine(req stubs.NodeArgs, response *stubs.NodeResp) (err error) {
   height := (req.YEnd - req.YStart) + 1
   newWorld := make([][]byte, height)
   for col := range newWorld {
       newWorld[col] = make([]byte, req.ImageWidth)
   }


   world := req.WorldSlice
   for y := req.YStart; y <= req.YEnd; y++ {
       for x := 0; x < req.ImageWidth; x++ {
           aliveNeighbors := 0
           for dy := -1; dy <= 1; dy++ {
               for dx := -1; dx <= 1; dx++ {
                   if dx == 0 && dy == 0 {
                       continue
                   }
                   neighborY := (y + dy + req.ImageWidth) % req.ImageWidth
                   neighborX := (x + dx + req.ImageWidth) % req.ImageWidth
                   if world[neighborY][neighborX] == 255 {
                       aliveNeighbors++
                   }
               }
           }
           cellValue := world[y-req.YStart][x]
           newValue := cellValue
           switch {
           case cellValue == 255 && (aliveNeighbors < 2 || aliveNeighbors > 3):
               newValue = 0
           case cellValue == 0 && aliveNeighbors == 3:
               newValue = 255
           }


           newWorld[y-req.YStart][x] = newValue
       }
   }
   response.WorldSlice = newWorld
   return nil
}


type Worker struct {
   wg       sync.WaitGroup
   listener net.Listener
   signal   chan string
   quitting bool
   broker   *rpc.Client
}


// Ensure clients have closed connections before closing server
func (w *Worker) serveConn(conn net.Conn) {
   w.wg.Add(1)
   defer w.wg.Done()
   rpc.ServeConn(conn)
}


func (w *Worker) startAccepting(listener net.Listener) {
   for {
       conn, err := listener.Accept()
       if err != nil {
           if w.quitting {
               return
           } else {
               fmt.Println("Accept error:", err)
           }
       } else {
           go w.serveConn(conn)
       }
   }
}


// subscribeToBroker handles broker subscription
func (w *Worker) subscribeToBroker(brokerAddr string) error {
   var err error
   w.broker, err = rpc.Dial("tcp", brokerAddr)
   if err != nil {
       return fmt.Errorf("failed to connect to broker: %v", err)
   }


   // Create subscription request
   subscriptionReq := stubs.Subscribe{
       IpAddress: w.listener.Addr().String(),
   }
   subscriptionRes := new(stubs.Status)


   // Subscribe to broker
   err = w.broker.Call("Broker.Subscribe", subscriptionReq, subscriptionRes)
   if err != nil {
       return fmt.Errorf("failed to subscribe to broker: %v", err)
   }


   return nil
}


func main() {
   // Parse command line flags
   pAddr := flag.String("port", "8040", "Port to listen on")
   brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker")
   flag.Parse()


   // Set up listener
   listener, err := net.Listen("tcp", ":"+*pAddr)
   if err != nil {
       fmt.Println("Error worker cannot connect to port:", err)
       return
   }


   // Initialize worker
   w := Worker{
       listener: listener,
       signal:   make(chan string, 1),
   }
   rpc.Register(&w)


   // Connect to broker
   err = w.subscribeToBroker(*brokerAddr)
   if err != nil {
       fmt.Printf("Failed to connect to broker: %v\n", err)
       listener.Close()
       return
   }


   fmt.Println("Server open on port", *pAddr)
   defer func() {
       listener.Close()
       if w.broker != nil {
           w.broker.Close()
       }
       close(w.signal)
   }()


   // Start accepting connections
   go w.startAccepting(listener)


   // Wait for kill signal
   <-w.signal
   fmt.Println("Server closing...")
   w.wg.Wait()
}




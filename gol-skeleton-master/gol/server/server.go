package main

import (
	"flag"
	"log"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolMasterRunner struct{}

var tickRequest = make(chan struct{}, 1)
var currentTurns = make(chan int, 1)
var cellCount = make(chan int, 1)

func main() {
	// List of worker node addresses (replace with IPs or DNS of your EC2 instances)
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	golMaster := new(GolMasterRunner)
	err := rpc.Register(golMaster)
	if err != nil {
		log.Fatalf("Error registering GolMasterRunner: %v", err)
	}

	// Start the listener
	listener, err := net.Listen("tcp", "0.0.0.0:"+*pAddr) // Bind to all interfaces
	if err != nil {
		log.Fatalf("Error starting listener: %v", err)
	}
	defer listener.Close()

	log.Printf("Server is listening on port %s...\n", *pAddr)

	// Accept incoming connections and handle RPC requests
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue // Skip the current connection and continue to listen for new ones
		}

		// Serve the connection using RPC
		log.Printf("Connected! connection: %v", conn)
		go rpc.ServeConn(conn)
	}
}

type WorkerResponse struct {
	FinalWorld     [][]byte // The updated world from the worker
	WorkerNumber   int
	TurnsCompleted int // The number of turns completed by the worker
}

func (g *GolMasterRunner) MasterStart(initReq stubs.InitialRequest, finalRes *stubs.FinalResponse) (err error) {
	workerNodes := []string{
		"ec2-34-192-200-75.compute-1.amazonaws.com:8040",
		"ec2-34-199-231-195.compute-1.amazonaws.com:8040",
		"ec2-35-153-27-128.compute-1.amazonaws.com:8040",
		"ec2-54-163-158-65.compute-1.amazonaws.com:8040",
	}

	var wg sync.WaitGroup
	passedWorld := initReq.NextWorld
	passedTurns := initReq.Turns
	passedThreads := 4
	sizeOfWorld := len(passedWorld)
	responseChannels := make([]chan [][]uint8, passedThreads)
	for i := 0; i < passedThreads; i++ {
		responseChannels[i] = make(chan [][]uint8)
	}

	for localTurns := 0; localTurns < passedTurns; localTurns++ {
		select {
		case <-tickRequest:
			go func() {
				currentTurns <- localTurns // Directly use localTurns
				cellCount <- len(calculateAliveCells(passedWorld))
				return
			}()
		default:
		}

		newWorld := make([][]byte, sizeOfWorld)
		for j := range newWorld {
			newWorld[j] = make([]byte, sizeOfWorld)
		}

		for k, workerAddr := range workerNodes {
			if workerAddr == "" {
				log.Printf("Worker address is empty. Skipping worker %d.\n", k)
				continue
			}

			wg.Add(1)
			go func(workerAddr string, index int) {
				defer wg.Done()
				client2, err := rpc.Dial("tcp", workerAddr)
				if err != nil {
					log.Printf("Failed to connect to worker %d: %v\n", index, err)
					return
				}

				req := stubs.Request{
					WorkerNumber: index,
					NextWorld:    passedWorld,
					Turns:        passedTurns,
					ThreadCount:  passedThreads,
				}
				res := new(stubs.Response)
				if err := client2.Call(stubs.StartWorker, req, res); err != nil {
					log.Printf("Error in worker %d RPC call: %v\n", index, err)
					return
				}
				responseChannels[index] <- res.FinalWorld
			}(workerAddr, k)
		}

		for j := 0; j < passedThreads; j++ {
			if j < len(responseChannels) {
				receivedWorld := <-responseChannels[j]
				for y := j * sizeOfWorld / passedThreads; y < (j+1)*sizeOfWorld/passedThreads; y++ {
					for x := 0; x < sizeOfWorld; x++ {
						newWorld[y][x] = receivedWorld[y][x]
					}
				}
			}
		}

		passedWorld = newWorld
	}

	// Wait for all goroutines to finish and close channels
	go func() {
		wg.Wait()
		for _, ch := range responseChannels {
			close(ch)
		}
	}()

	finalRes.FinalWorld = passedWorld
	finalRes.AliveCells = calculateAliveCells(passedWorld)
	finalRes.TurnsCompleted = initReq.Turns
	return
}

func (g *GolMasterRunner) TickTime(aliveRequest *stubs.AliveRequest, aliveCellResponse *stubs.AliveResponse) (err error) {
	tickRequest <- struct{}{}
	currentTurn2 := <-currentTurns
	cellCount2 := <-cellCount
	aliveCellResponse.AliveCellCount = cellCount2
	aliveCellResponse.CurrentTurns = currentTurn2
	return
}

func calculateAliveCells(world [][]byte) []util.Cell {
	size := len(world)
	aliveCollection := []util.Cell{}
	for y := 0; y < size; y++ { // Iterate over rows (height)
		for x := 0; x < size; x++ { // Iterate over columns (width)
			if world[y][x] == 255 { // Access as world[row][column] or world[y][x]
				alive := util.Cell{x, y}
				aliveCollection = append(aliveCollection, alive)
			}
		}
	}
	return aliveCollection
}

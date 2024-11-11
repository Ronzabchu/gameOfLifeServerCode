package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	go startIo(p, ioChannels{})
	c.ioCommand <- ioInput

	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	nextWorld := make([][]byte, p.ImageHeight)
	for i := range nextWorld {
		nextWorld[i] = make([]byte, p.ImageWidth)
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {

			pixelValue := <-c.ioInput
			nextWorld[y][x] = pixelValue
		}
	}
	client, _ := rpc.Dial("tcp", "ec2-3-224-160-246.compute-1.amazonaws.com:8030")
	defer client.Close()
	responseChannel := make(chan stubs.FinalResponse)
	//done := make(chan bool)

	//requestChan := make(chan struct{})
	// Channel for request signals

	//var ticker *time.Ticker
	//ticker = time.NewTicker(2 * time.Second)
	//go background(requestChan, p, c, currentWorld, currentTurn, done, ticker)
	go keyListener(c, client, p)

	turn := 0
	c.events <- StateChange{turn, Executing}

	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)

	// TODO: Execute all turns of the Game of Life.
	values2 := makeCall(client, nextWorld, p.Turns, p.Threads, responseChannel)
	nukeAlive := values2.AliveCells
	nukeCompletedTurns := values2.TurnCompleted
	nukeFinalWorld := values2.World

	// TODO: Report the final state using FinalTurnCompleteEvent.
	//close(done)

	c.events <- FinalTurnComplete{CompletedTurns: nukeCompletedTurns, Alive: nukeAlive}

	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, nukeCompletedTurns)

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {

			pixelValue := nukeFinalWorld[y][x]
			c.ioOutput <- pixelValue
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Make sure that the Io has finished any output before exiting.

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}

type Value struct {
	World         [][]byte
	TurnCompleted int
	AliveCells    []util.Cell
}

func makeCall(client *rpc.Client, worldProcess [][]byte, turns int, threads int, responseChannel chan stubs.FinalResponse) Value {
	request := stubs.InitialRequest{NextWorld: worldProcess, Turns: turns, ThreadCount: threads}
	if client == nil {
		log.Fatal("RPC client is nil. Could not connect to the server.")
	}

	response := new(stubs.FinalResponse)

	err := client.Call(stubs.StartMaster, request, response)
	if err != nil {
		log.Fatalf("Error during RPC call: %v", err)
	}

	fmt.Printf("passingBack from server.\n")

	return Value{AliveCells: response.AliveCells, World: response.FinalWorld, TurnCompleted: response.TurnsCompleted}
}

func sendCommand(client *rpc.Client, command string) Value {
	request := stubs.CommandRequest{command}
	response := new(stubs.FinalResponse)
	err := client.Call(stubs.DoCommand, request, response)
	if err != nil {
		log.Fatalf("Error during RPC call: %v", err)
	}
	return Value{response.FinalWorld, response.TurnsCompleted, response.AliveCells}
}

func keyListener(c distributorChannels, client *rpc.Client, p Params) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)
	playState := "play"
	for {
		select {
		case keyPressed := <-c.keyPresses:
			if keyPressed == 'p' && playState == "play" {
				playState = "pause"
				value := sendCommand(client, "PAUSE")
				c.events <- StateChange{value.TurnCompleted, Paused}
			} else if keyPressed == 'p' && playState == "pause" {
				playState = "play"
				value := sendCommand(client, "PLAY")
				c.events <- StateChange{value.TurnCompleted, Executing}
			} else if keyPressed == 'q' {
				value := sendCommand(client, "QUIT")
				c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, value.TurnCompleted)

				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {

						pixelValue := value.World[y][x]
						c.ioOutput <- pixelValue
					}
				}

				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: value.TurnCompleted, Filename: filename}
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- StateChange{value.TurnCompleted, Quitting}

			} else if keyPressed == 's' {
				value := sendCommand(client, "SAVE")
				c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, value.TurnCompleted)

				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {

						pixelValue := value.World[y][x]
						c.ioOutput <- pixelValue
					}
				}

				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: value.TurnCompleted, Filename: filename}
			}
		}
	}
}

func background(requestChan chan struct{}, p Params, c distributorChannels, done chan bool, ticker *time.Ticker) {
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-done:
			ticker.Stop()
			return
		case <-ticker.C:
			requestChan <- struct{}{}
			//c.events <- AliveCellsCount{CompletedTurns: currentTurn2, CellsCount: 0}
		}
	}
}

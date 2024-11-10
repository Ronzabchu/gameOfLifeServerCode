package gol

import (
	"fmt"
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

type Value struct {
	World [][]byte
	Turn  int
}

type Value2 struct {
	Alive []util.Cell
	Turn  int
}

func makeCallWorld(client *rpc.Client, world [][]byte, p Params) Value {
	request := stubs.Request{world, p.Turns, p.ImageWidth}
	response := new(stubs.Response)
	client.Call(stubs.TurnChanger, request, response)
	return Value{response.World, response.Turn}
}

func makeCommandWorld(client *rpc.Client, command string) Value {
	response := new(stubs.Response)
	client.Call(stubs.CommandHandler, command, response)
	return Value{response.World, response.Turn}
}

func makeAliveCall(client *rpc.Client) Value2 {
	response := new(stubs.AliveCellsResponse)
	client.Call(stubs.AliveCells, nil, response)
	return Value2{response.Cells, response.Turn}
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

	flippedCells := []util.Cell{}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {

			pixelValue := <-c.ioInput
			if pixelValue == 255 {
				flipped := util.Cell{x, y}
				flippedCells = append(flippedCells, flipped)
			}
			nextWorld[y][x] = pixelValue
		}
	}

	currentWorld := make(chan [][]uint8)
	currentTurn := make(chan int)
	done := make(chan bool)
	// Channel for request signals

	conn, err := rpc.Dial("tcp", "127.0.0.1:8030")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	fmt.Println("Connected to server")
	defer conn.Close()

	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)

	var ticker *time.Ticker
	ticker = time.NewTicker(2 * time.Second)
	go background(conn, c, done, ticker)
	go keyListener(c, conn, p)

	turn := 0
	//c.events <- CellsFlipped{turn, flippedCells}
	c.events <- StateChange{turn, Executing}

	finalWorld := makeCallWorld(conn, nextWorld, p)

	testQuit := false

	// TODO: Execute all turns of the Game of Life.

	var nuke = calculateAliveCells(p, finalWorld.World)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	close(done)
	close(currentTurn)
	close(currentWorld)

	c.events <- FinalTurnComplete{CompletedTurns: finalWorld.Turn, Alive: nuke}
	if testQuit == false {
		//c.events <- FinalTurnComplete{CompletedTurns: turn + 1, Alive: nuke}
		c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	} else {

		//c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: nuke}
		//c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {

			pixelValue := finalWorld.World[y][x]
			c.ioOutput <- pixelValue
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{CompletedTurns: finalWorld.Turn, Filename: filename}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Make sure that the Io has finished any output before exiting.

	c.events <- StateChange{finalWorld.Turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}

func keyListener(c distributorChannels, client *rpc.Client, p Params) {
	playState := "play"
	for {
		select {
		case keyPressed := <-c.keyPresses:
			fmt.Println("key received")
			if keyPressed == 'p' && playState == "play" {
				makeCommandWorld(client, "PAUSE")
				playState = "pause"
				fmt.Println("paused")
			} else if keyPressed == 'p' && playState == "pause" {
				makeCommandWorld(client, "PLAY")
				playState = "play"
				fmt.Println("playing")
			} else if keyPressed == 'q' {
				value := makeCommandWorld(client, "QUIT")
				fmt.Println("quitting")
				for y := 0; y < len(value.World); y++ {
					for x := 0; x < len(value.World); x++ {

						pixelValue := value.World[y][x]
						c.ioOutput <- pixelValue
					}
				}

				c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: value.Turn, Filename: filename}
				return
			} else if keyPressed == 's' {
				value := makeCommandWorld(client, "SAVE")
				fmt.Println("saving")
				for y := 0; y < len(value.World); y++ {
					for x := 0; x < len(value.World); x++ {

						pixelValue := value.World[y][x]
						c.ioOutput <- pixelValue
					}
				}

				c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Threads)
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: value.Turn, Filename: filename}
			}
		}
	}
}

func background(client *rpc.Client, c distributorChannels, done chan bool, ticker *time.Ticker) {
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-done:
			ticker.Stop()
			return
		case t := <-ticker.C:
			fmt.Println("poo")
			value := makeAliveCall(client)
			c.events <- AliveCellsCount{CompletedTurns: value.Turn, CellsCount: len(value.Alive)}
			fmt.Println(t)
		}
	}
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	aliveCollection := []util.Cell{}
	for y := 0; y < p.ImageHeight; y++ { // Iterate over rows (height)
		for x := 0; x < p.ImageWidth; x++ { // Iterate over columns (width)
			if world[y][x] == 255 { // Access as world[row][column] or world[y][x]
				alive := util.Cell{x, y}
				aliveCollection = append(aliveCollection, alive)
			}
		}
	}
	return aliveCollection
}

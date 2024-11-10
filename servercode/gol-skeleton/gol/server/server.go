package main

import (
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GameOfLifeServer struct{}

var globalTurn = 0
var globalWorld [][]byte

func main() {

	rpc.Register(&GameOfLifeServer{})
	ln, err := net.Listen("tcp", ":8030")
	if err != nil {
		handleError(err)
		return
	}
	defer ln.Close()
	rpc.Accept(ln)

}

func (s *GameOfLifeServer) HandleCommand(req stubs.RequestCommand, res *stubs.Response) (err error) {
	if req.Command == "PAUSE" {

	} else if req.Command == "PLAY" {

	} else if req.Command == "SAVE" {
		res.World = globalWorld
		res.Turn = globalTurn
	} else if req.Command == "QUIT" {
		res.World = globalWorld
		res.Turn = globalTurn
	}
	return
}

func (s *GameOfLifeServer) NumberOfTurns(req stubs.Request, res *stubs.Response) (err error) {
	world := req.World
	dimensions := req.Dimensions
	if req.Turns != 0 {
		for i := 0; i < req.Turns; i++ {
			world = calculateNextState(world, dimensions)
			globalTurn = req.Turns
			globalWorld = world
			res.Turn = i
			res.World = world
		}
	} else {
		res.World = world
		res.Turn = 0
	}
	return

}

func calculateNextState(world [][]byte, dimensions int) [][]byte {
	// Create a new grid to store the next state of the world
	nextWorld := make([][]byte, dimensions)
	for i := range nextWorld {
		nextWorld[i] = make([]byte, dimensions)
	}

	//flippedCells := []util.Cell{}

	// Iterate over each cell in the grid
	for y := 0; y < dimensions; y++ {
		for x := 0; x < dimensions; x++ {
			sum := 0
			// Iterate over the 3x3 neighborhood
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					// Skip the center cell itself (y, x)
					if i == 0 && j == 0 {
						continue
					}

					// Calculate wrapped coordinates using modulo
					ny := (y + i + dimensions) % dimensions
					nx := (x + j + dimensions) % dimensions

					// Sum the neighbor value (wrapped around)
					if world[ny][nx] == 255 {
						sum++
					}
				}
			}

			// Apply the Game of Life rules to the current cell
			if world[y][x] == 255 { // Cell is alive
				if sum < 2 || sum > 3 {
					nextWorld[y][x] = 0 // Cell dies
					//flipped := util.Cell{x, y}
					//flippedCells = append(flippedCells, flipped)
				} else {
					nextWorld[y][x] = 255 // Cell stays alive
				}
			} else { // Cell is dead
				if sum == 3 {
					nextWorld[y][x] = 255 // Cell becomes alive
					//flipped := util.Cell{x, y}
					//flippedCells = append(flippedCells, flipped)
				} else {
					nextWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	// Return the next state of the world
	return nextWorld

}

func (s *GameOfLifeServer) calculateAliveCells(res *stubs.AliveCellsResponse) (err error) {
	aliveCollection := []util.Cell{}
	world := globalWorld
	for y := 0; y < len(world); y++ { // Iterate over rows (height)
		for x := 0; x < len(world); x++ { // Iterate over columns (width)
			if world[y][x] == 255 { // Access as world[row][column] or world[y][x]
				alive := util.Cell{x, y}
				aliveCollection = append(aliveCollection, alive)
			}
		}
	}
	res.Cells = aliveCollection
	res.Turn = globalTurn
	return
}

func handleError(err error) {
	// TODO: all
	// Deal with an error event.
	fmt.Println("Error:", err)
}

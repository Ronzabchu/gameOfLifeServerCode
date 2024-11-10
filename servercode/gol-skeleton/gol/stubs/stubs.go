package stubs

import "uk.ac.bris.cs/gameoflife/util"

var TurnChanger = "GameOfLifeServer.NumberOfTurns"
var CommandHandler = "GameOfLifeServer.HandleCommand"
var AliveCells = "GameOfLifeServer.AliveCells"

type Response struct {
	World [][]byte
	Turn  int
}

type AliveCellsResponse struct {
	Cells []util.Cell
	Turn  int
}

type Request struct {
	World      [][]byte
	Turns      int
	Dimensions int
}

type RequestCommand struct {
	Command string
}

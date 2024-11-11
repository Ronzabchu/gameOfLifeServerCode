package stubs

import "uk.ac.bris.cs/gameoflife/util"

var StartMaster = "GolMasterRunner.MasterStart"
var StartWorker = "GameOfLifeOperations.ProcessGameOfLife"
var RunTicker = "GolMasterRunner.TickTime"

type Response struct {
	WorkerNumber   int
	FinalWorld     [][]byte
	TurnsCompleted int
}

type AliveResponse struct {
	AliveCellCount int
	CurrentTurns   int
}

type AliveRequest struct {
	TimeToRequest bool
}

type Request struct {
	WorkerNumber int
	NextWorld    [][]byte
	Turns        int
	ThreadCount  int
}

type FinalResponse struct {
	FinalWorld     [][]byte
	TurnsCompleted int
	AliveCells     []util.Cell
}

type InitialRequest struct {
	NextWorld   [][]byte
	Turns       int
	ThreadCount int
}

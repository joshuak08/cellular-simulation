package bStubs

var BTurnHandler = "GolOperations.CalculateNextWorld"
var BAliveHandler = "GolOperations.CalculateAlive"
var BSnapshotHandler = "GolOperations.Snapshot"
var BShutHandler = "GolOperations.ShutServer"

type Response struct {
	Turns      int
	World      [][]byte
	AliveCells int
	//IoCommand chan<- gol.TurnComplete
}

type Request struct {
	World  [][]byte
	Width  int
	StartY int
	EndY   int
	Height int
	Turns  int
	Kill   bool
}

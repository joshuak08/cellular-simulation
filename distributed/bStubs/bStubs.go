package bStubs

var BTurnHandler = "GolOperations.CalculateNextWorld"
var BShutHandler = "GolOperations.ShutServer"

type Response struct {
	Turns      int
	World      [][]byte
	AliveCells int
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

package stubs

var TurnHandler = "GolOperations.CalculateNextWorld"
var AliveHandler = "GolOperations.CalculateAlive"

type Response struct {
	Turns      int
	World      [][]byte
	AliveCells int
}

type Request struct {
	World  [][]byte
	Width  int
	Height int
	Turns  int
}

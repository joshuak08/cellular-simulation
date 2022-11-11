package stubs

var TurnHandler = "GolOperations.CalculateNextWorld"

type Response struct {
	Turns int
	World [][]byte
}

type Request struct {
	World  [][]byte
	Width  int
	Height int
	Turns  int
}

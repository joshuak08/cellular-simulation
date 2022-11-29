package stubs

var TurnHandler = "Broker.CalculateNextWorld"
var AliveHandler = "Broker.CalculateAlive"
var SnapshotHandler = "Broker.Snapshot"
var ShutHandler = "Broker.ShutServer"

type Response struct {
	Turns      int
	World      [][]byte
	AliveCells int
}

type Request struct {
	World [][]byte
	Width int
	//StartY int
	//EndY   int
	Height int
	Turns  int
	Kill   bool
}

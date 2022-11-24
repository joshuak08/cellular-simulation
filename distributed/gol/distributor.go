package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
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

const alive = 255
const dead = 0

func makeCallWorld(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) *stubs.Response {
	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false}
	response := new(stubs.Response)
	client.Call(stubs.TurnHandler, request, response)
	return response
}

func makeCallAliveCells(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) *stubs.Response {
	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false}
	response := new(stubs.Response)
	client.Call(stubs.AliveHandler, request, response)
	return response
}

func makeCallSnapshot(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) *stubs.Response {
	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false}
	response := new(stubs.Response)
	client.Call(stubs.SnapshotHandler, request, response)
	return response
}

func closeServer(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) {
	request := stubs.Request{world, ImageWidth, ImageHeight, Turns, true}
	response := new(stubs.Response)
	client.Call(stubs.ShutHandler, request, response)
	return
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	c.ioCommand <- ioInput
	// Create filename from parameters and send down the filename channel
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	// Receive image byte by byte and store in 2d world
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}

	//turn := 0

	// TODO: Execute all turns of the Game of Life.
	done := make(chan bool)

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer client.Close()

	ticker := time.NewTicker(2 * time.Second)
	//block := make(chan bool)
	pPressed := false

	go func() {
		for {
			select {
			case key := <-keyPresses:
				snapshot := makeCallSnapshot(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
				switch key {
				case 's':
					c.ioCommand <- ioOutput
					outfile := strconv.Itoa(snapshot.Turns)
					c.ioFilename <- outfile
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							c.ioOutput <- snapshot.World[i][j]
						}
					}
					c.events <- ImageOutputComplete{snapshot.Turns, outfile}
				case 'q':
					c.ioCommand <- ioOutput
					outfile := strconv.Itoa(snapshot.Turns)
					c.ioFilename <- outfile
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}
					fmt.Println("Quitting")
					c.events <- ImageOutputComplete{snapshot.Turns, outfile}
					c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
				case 'p':
					c.events <- StateChange{snapshot.Turns, Paused}
					pPressed = true
					for {
						switch <-keyPresses {
						case 'p':
							c.events <- StateChange{snapshot.Turns, Executing}
							fmt.Println("Continuing")
							pPressed = false
						}
						if !pPressed {
							break
						}
					}
				case 'k':
					c.ioCommand <- ioOutput
					outfile := strconv.Itoa(snapshot.Turns)
					c.ioFilename <- outfile
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}
					fmt.Println("Quitting and killing server")
					c.events <- ImageOutputComplete{snapshot.Turns, outfile}
					closeServer(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
					c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
				}
			case <-done:
				return
			case <-ticker.C:
				response := makeCallAliveCells(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
				cells := AliveCellsCount{response.Turns, response.AliveCells}
				c.events <- cells
			}
		}
	}()

	// TODO: RPC Client code

	//server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	//flag.Parse()

	response := makeCallWorld(client, world, p.ImageWidth, p.ImageHeight, p.Turns)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.ioCommand <- ioOutput

	//Create output file from filename and current turn send down the filename channel
	outfile := filename + "x" + strconv.Itoa(response.Turns)
	c.ioFilename <- outfile

	//// Send image byte by byte to output
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- response.World[i][j]
		}
	}

	c.events <- ImageOutputComplete{response.Turns, outfile}
	last := FinalTurnComplete{CompletedTurns: response.Turns, Alive: calculateAliveCells(p, response.World)}
	// Tick until final turn
	done <- true
	c.events <- last

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{response.Turns, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == alive {
				newCell := util.Cell{X: j, Y: i}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

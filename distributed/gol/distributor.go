package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"sync"
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

var mu sync.Mutex

// RPC call function from client to broker to calculate next state of world
func makeCallWorld(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns, Threads int) *stubs.Response {
	request := stubs.Request{world, ImageWidth, ImageHeight, Turns, false, Threads}
	response := new(stubs.Response)
	client.Call(stubs.TurnHandler, request, response)
	return response
}

// RPC call function from client to broker to retrieve number of alive cells and turns in current world
func makeCallAliveCells(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns, Threads int) *stubs.Response {
	request := stubs.Request{world, ImageWidth, ImageHeight, Turns, false, Threads}
	response := new(stubs.Response)
	client.Call(stubs.AliveHandler, request, response)
	return response
}

// RPC call function to retrieve current world and turns to output into a pgm file
func makeCallSnapshot(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns, Threads int) *stubs.Response {
	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false, Threads}
	response := new(stubs.Response)
	client.Call(stubs.SnapshotHandler, request, response)
	return response
}

// RPC call function to close all worker servers and broker
func closeServer(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns, Threads int) {
	request := stubs.Request{world, ImageWidth, ImageHeight, Turns, true, Threads}
	response := new(stubs.Response)
	client.Call(stubs.ShutHandler, request, response)
	return
}

// Calculates number of alive cells in the world after each iteration, it returns a slice with type util.Cell
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

// Function to create world and initialise state from input
func createWorld(p Params, c distributorChannels) [][]byte {
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
	return world
}

// Outputs image into ioOutput and sends event imageOutputComplete to events channel
func outImage(p Params, c distributorChannels, snapshot *stubs.Response) {
	// Sets command to output
	c.ioCommand <- ioOutput
	outfile := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(snapshot.Turns)
	// Write file name
	c.ioFilename <- outfile
	// Outputs file byte by byte
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- snapshot.World[i][j]
		}
	}
	// Notify events channel that image output done, with relevant turns and filename
	c.events <- ImageOutputComplete{snapshot.Turns, outfile}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	c.ioCommand <- ioInput
	// Create filename from parameters and send down the filename channel
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	// TODO: Create a 2D slice to store the world.
	world := createWorld(p, c)

	// TODO: Execute all turns of the Game of Life.

	// Dials to broker
	broker, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer broker.Close()

	// Ticker that ticks every 2s to count number of alive cells
	ticker := time.NewTicker(2 * time.Second)
	// Bool value to determine if execution is paused or running
	pPressed := false
	// Bool channel to exit out of the following go routine when it is execution is done
	done := make(chan bool)
	key := 'a'

	// Goroutine to check if any keys pressed, ticker is ticking, or
	go func() {
		for {
			select {
			// Receives keys pressed
			case key = <-keyPresses:
				// Calls to receive current world to be saved into a pgm file
				snapshot := makeCallSnapshot(broker, world, p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
				switch key {
				// save image
				case 's':
					outImage(p, c, snapshot)
				// client quits and disconnects
				case 'q':
					c.events <- StateChange{snapshot.Turns, Quitting}
					outImage(p, c, snapshot)
					fmt.Println("Quitting")
					c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
				// execution paused
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
				// Client kills broker and servers shuts whole system down
				case 'k':
					outImage(p, c, snapshot)
					fmt.Println("Quitting and killing server")
					closeServer(broker, world, p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
					c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
					c.events <- StateChange{snapshot.Turns, Quitting}
					close(c.events)
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					return
				}
			// When done receives a true value, it returns out of this go routine function
			case <-done:
				return
			// When ticker ticks every 2s
			case <-ticker.C:
				// Makes rpc call function to retrieve num of alive cells
				tick := makeCallAliveCells(broker, world, p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
				cells := AliveCellsCount{tick.Turns, tick.AliveCells}
				// Sends it down events channel to update num of alive cells
				c.events <- cells
			}
		}
	}()

	// Retrieves response that contains world number of alive cells, turns completed
	mu.Lock()
	response := makeCallWorld(broker, world, p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
	mu.Unlock()
	// TODO: RPC Client code

	// TODO: Report the final state using FinalTurnCompleteEvent.
	// if k pressed return
	if key == 'k' {
		return
	}
	// Outputs world
	outImage(p, c, response)
	last := FinalTurnComplete{CompletedTurns: response.Turns, Alive: calculateAliveCells(p, response.World)}
	// Tick until final turn
	done <- true
	// Sends FinalTurnComplete event to events channel
	c.events <- last

	c.events <- StateChange{response.Turns, Quitting}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

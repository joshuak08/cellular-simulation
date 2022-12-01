package gol

import (
	"fmt"
	"strconv"
	"time"
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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	c.ioCommand <- ioInput
	// Create filename from parameters and send down the filename channel
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	// TODO: Create a 2D slice to store the world.
	world := makeWorld(p.ImageHeight, p.ImageWidth)

	// Receive image byte by byte and store in 2d world
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			val := <-c.ioInput
			world[i][j] = val
			// Initialises starting state of world and sent down events channel with event CellFlipped
			if val == alive {
				aliveCell := util.Cell{X: j, Y: i}
				c.events <- CellFlipped{CompletedTurns: 0, Cell: aliveCell}
			}
		}
	}

	// TODO: Execute all turns of the Game of Life.

	// Ticker that ticks every 2s to count number of alive cells
	ticker := time.NewTicker(2 * time.Second)
	turn := 0
	// Bool value to determine if execution is paused or running
	pPressed := false
	// Runs for input number of turns
	for turn < p.Turns {
		// Creates immutable world
		immutableWorld := makeImmutableWorld(world)

		out := make([]chan [][]byte, p.Threads)
		for i := range out {
			out[i] = make(chan [][]byte)
		}

		var newPixelData [][]uint8
		for i := 0; i < p.Threads; i++ {
			go worker(p, immutableWorld, i*p.ImageHeight/p.Threads, (i+1)*p.ImageHeight/p.Threads, out[i], c, turn)
		}

		for i := 0; i < len(out); i++ {
			newPixelData = append(newPixelData, <-out[i]...)
		}

		world = newPixelData
		turn++
		c.events <- TurnComplete{turn}

		// Stops executing the next world state and outputs last saved world
		// if something is received from keyPresses channel or ticker
		select {
		// When ticker ticks every 2s send event to events channel
		case <-ticker.C:
			c.events <- AliveCellsCount{turn, len(calculateAliveCells(p, world))}
		// Receives keys pressed
		case key := <-keyPresses:
			switch key {
			// outputs world and saves it as a file
			case 's':
				c.events <- StateChange{turn, Executing}
				outImage(p, world, c, turn)
			// quits function and outputs world and saves it
			case 'q':
				c.events <- StateChange{turn, Quitting}
				outImage(p, world, c, turn)
				c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}
			// Pauses it if p not pressed and continues if pressed
			case 'p':
				c.events <- StateChange{turn, Paused}
				pPressed = true
				for {
					switch <-keyPresses {
					case 'p':
						c.events <- StateChange{turn, Executing}
						fmt.Println("Continuing")
						pPressed = false
					}
					if !pPressed {
						break
					}
				}
			}
		}
	}

	// Create output file from filename and current turn send down the filename channel
	outImage(p, world, c, turn)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

// Outputs image into ioOutput and notifies events channel that image output complete
func outImage(p Params, world [][]byte, c distributorChannels, turn int) {
	// Sets command to output
	c.ioCommand <- ioOutput
	outfile := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
	// Write file name
	c.ioFilename <- outfile
	// Outputs file byte by byte
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
	// Notify events channel that image output done, with relavant turns and filename
	c.events <- ImageOutputComplete{turn, outfile}
}

// makeMatrix makes and returns a 2D slice with the given dimensions.
func makeWorld(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

// creates an Immutable world
func makeImmutableWorld(world [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return world[y][x]
	}
}

// Worker function to distribute the execution to multiple threads using go routines, returns output using channels
func worker(p Params, immutableWorld func(y, x int) byte, startY int, endY int, tempWorld chan<- [][]uint8, c distributorChannels, turn int) {
	calculatedSlice := calculateNextState(p, immutableWorld, startY, endY, c, turn)
	tempWorld <- calculatedSlice
}

// GoL logic to calculate next state for the world, that returns a 2d slice
func calculateNextState(p Params, immutableWorld func(y, x int) byte, startY int, endY int, c distributorChannels, turn int) [][]byte {
	// newGrid creates a new slice that will return the new world, with height that is proportionately separated with other nodes
	height := endY - startY
	newGrid := make([][]byte, height)
	for i := range newGrid {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, p.ImageWidth)
	}
	// Calculate world in current slice.
	// Compare the cell with the one in the old world
	for i := startY; i < endY; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			// Counts number of neighbours for each cell
			neighbors := countNeighbours(p, j, i, immutableWorld)
			if immutableWorld(i, j) == alive {
				if neighbors == 2 || neighbors == 3 {
					newGrid[i-startY][j] = alive
				} else {
					newGrid[i-startY][j] = dead
					c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
				}
			}

			if immutableWorld(i, j) == dead {
				if neighbors == 3 {
					newGrid[i-startY][j] = alive
					c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
				} else {
					newGrid[i-startY][j] = dead
				}
			}
		}
	}
	return newGrid
}

// Counts the number of neighbours for each cell/entry
func countNeighbours(p Params, x, y int, immutableWorld func(y, x int) byte) int {
	var aliveCount = 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			// Don't count self as neighbour
			if i == 0 && j == 0 {
				continue
			}
			// Wraparound. Add height and width for negative values
			r := (x + i + p.ImageWidth) % p.ImageWidth
			c := (y + j + p.ImageHeight) % p.ImageHeight
			if immutableWorld(c, r) == alive {
				aliveCount++
			}
		}
	}
	return aliveCount
}

// Calculates number of alive cells in the world after each iteration, it returns a slice with type util.Cell
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	// Iterate through every cell and count number of alive cells to update
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			// If cell is alive add it to slice of alive cells with coordinates
			if world[i][j] == alive {
				newCell := util.Cell{X: j, Y: i}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

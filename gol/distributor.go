package gol

import (
	"fmt"
	"strconv"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

type cell struct {
	x, y int
}

const alive = 255
const dead = 0

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	c.ioCommand <- ioInput
	// Create filename from parameters and send down the filename channel
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageHeight)
	fmt.Println("File sent down channel", filename)
	c.ioFilename <- filename
	fmt.Println("Debug")
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

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == alive {
				cellFlip := util.Cell{X: j, Y: i}
				c.events <- CellFlipped{turn, cellFlip}
			}
		}
	}

	for ; turn < p.Turns; turn++ {
		world = calculateNextState(p, world)
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	var aliveCell []util.Cell
	for i := 0; i < len(world); i++ {
		for j := 0; j < len(world[i]); j++ {
			if world[i][j] == alive {
				aliveCell = append(aliveCell, util.Cell{X: j, Y: i})
			}
		}
	}

	last := FinalTurnComplete{CompletedTurns: turn, Alive: aliveCell}
	c.events <- last

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func calculateNextState(p Params, world [][]byte) [][]byte {
	// Make allocates an array and returns a slice that refers to that array
	newGrid := make([][]byte, p.ImageHeight)
	for i := range world {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, p.ImageWidth)
	}

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			neighbours := countNeighbours(p, i, j, world)
			state := world[i][j]
			if state == dead && neighbours == 3 {
				newGrid[i][j] = alive
			} else if state == alive && (neighbours < 2 || neighbours > 3) {
				newGrid[i][j] = dead
			} else {
				newGrid[i][j] = state
			}
		}
	}
	return newGrid
}

func countNeighbours(p Params, x, y int, world [][]byte) int {
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
			if world[r][c] == alive {
				aliveCount++
			}
		}
	}
	return aliveCount
}

func calculateAliveCells(p Params, world [][]byte) []cell {
	var aliveCells []cell
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == alive {
				newCell := cell{x: j, y: i}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

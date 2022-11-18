package gol

import (
	"fmt"
	"strconv"
	"sync"
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

var mu sync.Mutex

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
			if val == alive {
				aliveCell := util.Cell{X: j, Y: i}
				c.events <- CellFlipped{CompletedTurns: 0, Cell: aliveCell}
			}
		}
	}

	// TODO: Execute all turns of the Game of Life.

	ticker := time.NewTicker(2 * time.Second)
	turn := 0
	pPressed := false
	for turn < p.Turns {
		//fmt.Println("TEST")

		immutableWorld := makeImmutableWorld(world)

		out := make([]chan [][]byte, p.Threads)
		for i := range out {
			out[i] = make(chan [][]byte)
		}

		//workerHeight := p.ImageHeight / p.Threads
		var newPixelData [][]uint8
		for i := 0; i < p.Threads; i++ {
			go worker(p, immutableWorld, i*p.ImageHeight/p.Threads, (i+1)*p.ImageHeight/p.Threads, out[i], c, turn)
		}

		//newPixelData := makeWorld(0, 0)
		for i := 0; i < len(out); i++ {
			newPixelData = append(newPixelData, <-out[i]...)
		}

		world = newPixelData
		turn++
		c.events <- TurnComplete{turn}

		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{turn, len(calculateAliveCells(p, world))}
		case key := <-keyPresses:
			switch key {
			case 's':
				c.events <- StateChange{turn, Executing}
				outImage(p, world, c, turn)
			case 'q':
				c.events <- StateChange{turn, Quitting}
				outImage(p, world, c, turn)
				c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}
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
		default:
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

func outImage(p Params, world [][]byte, c distributorChannels, turn int) {
	c.ioCommand <- ioOutput
	outfile := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
	c.ioFilename <- outfile
	// Send image byte by byte to output
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
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

func makeImmutableWorld(world [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return world[y][x]
	}
}

func worker(p Params, immutableWorld func(y, x int) byte, startY int, endY int, tempWorld chan<- [][]uint8, c distributorChannels, turn int) {
	calculatedSlice := calculateNextState(p, immutableWorld, startY, endY, c, turn)
	tempWorld <- calculatedSlice
}

func calculateNextState(p Params, immutableWorld func(y, x int) byte, startY int, endY int, c distributorChannels, turn int) [][]byte {
	// Make allocates an array and returns a slice that refers to that array
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

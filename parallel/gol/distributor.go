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

	// Notify GUI about a change of state of a single cell.
	// Send this event for all cells that are alive when the image is loaded in.
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == alive {
				cellFlip := util.Cell{X: j, Y: i}
				c.events <- CellFlipped{turn, cellFlip}
			}
		}
	}

	block := make(chan bool)
	pPressed := false
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case key := <-keyPresses:
				switch key {
				case 's':
					c.ioCommand <- ioOutput
					outfile := strconv.Itoa(turn)
					c.ioFilename <- outfile
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}
					c.events <- ImageOutputComplete{turn, outfile}
				case 'q':
					c.ioCommand <- ioOutput
					outfile := strconv.Itoa(turn)
					c.ioFilename <- outfile
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}
					c.events <- ImageOutputComplete{turn, outfile}
					c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}
				case 'p':
					if pPressed == true {
						fmt.Println("Continuing")
						block <- false
					} else {
						fmt.Println("Paused", turn+1)
					}
					// Flip the p switch (pause/unpause)
					pPressed = !pPressed
				}
			case <-done:
				return
			case <-ticker.C:
				// Only one goroutine (worker) can notify user about currently alive cells at once
				cells := AliveCellsCount{turn, len(calculateAliveCells(p, world))}
				c.events <- cells
			}
		}
	}()

	for ; turn < p.Turns; turn++ {
		if pPressed {
			<-block
		}
		var newPixelData [][]uint8
		if p.Threads == 1 {
			world = calculateNextState(p, world, 0, p.ImageHeight)
		} else {
			var slice []chan [][]uint8
			for i := 0; i < p.Threads; i++ {
				slice = append(slice, make(chan [][]uint8))
			}
			for i := 0; i < p.Threads; i++ {
				go worker(p, world, i*p.ImageHeight/p.Threads, (i+1)*p.ImageHeight/p.Threads, slice[i])
			}
			for i := 0; i < len(slice); i++ {
				newPixelData = append(newPixelData, <-slice[i]...)
			}

			// Notify GUI if the new cell is no longer the same state as the old cell
			for i := 0; i < p.ImageHeight; i++ {
				for j := 0; j < p.ImageWidth; j++ {
					if newPixelData[i][j] != world[i][j] {
						cellFlip := util.Cell{X: j, Y: i}
						c.events <- CellFlipped{turn, cellFlip}
					}
				}
			}

			// Swap old world with new world once the alive cells count has been reported
			world = newPixelData
		}
		c.events <- TurnComplete{turn}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.ioCommand <- ioOutput

	// Create output file from filename and current turn send down the filename channel
	outfile := filename + "x" + strconv.Itoa(turn)
	c.ioFilename <- outfile

	// Send image byte by byte to output
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}

	c.events <- ImageOutputComplete{turn, filename}
	last := FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
	// Tick until final turn
	done <- true
	c.events <- last

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func worker(p Params, world [][]byte, startY int, endY int, out chan<- [][]uint8) {
	out <- calculateNextState(p, world, startY, endY)
}

func calculateNextState(p Params, world [][]byte, startY int, endY int) [][]byte {
	// Make allocates an array and returns a slice that refers to that array
	height := endY - startY
	newGrid := make([][]byte, height)
	for i := range newGrid {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, p.ImageWidth)
	}
	for i := startY; i < endY; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			neighbours := countNeighbours(p, i, j, world)
			state := world[i][j]
			if state == dead && neighbours == 3 {
				newGrid[i-startY][j] = alive
			} else if state == alive && (neighbours < 2 || neighbours > 3) {
				newGrid[i-startY][j] = dead
			} else {
				newGrid[i-startY][j] = state
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

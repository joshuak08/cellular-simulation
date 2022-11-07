package gol

import "strconv"

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

const alive = 255
const dead = 0

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// Create filename from parameters and send down the filename channel
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageHeight)
	println(filename)
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

	// TODO: Report the final state using FinalTurnCompleteEvent.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func calculateNextState(p Params, world [][]byte) [][]byte {
	grid := make([][]byte, p.ImageHeight)
	for i := range world {
		grid[i] = make([]byte, p.ImageWidth)
	}

	for x := 0; x < p.ImageWidth; x++ {
		for y := 0; y < p.ImageHeight; y++ {
			count := countAlive(p, x, y, world)
			state := world[x][y]
			if state == dead && count == 3 {
				grid[x][y] = alive
			} else if state == alive && (count < 2 || count > 3) {
				grid[x][y] = dead
			} else {
				grid[x][y] = state
			}
		}
	}

	return grid
}

func countAlive(p Params, x int, y int, world [][]byte) int {
	count := 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i == 0 && j == 0 {
				continue
			}

			b := (x + i + p.ImageWidth) % p.ImageWidth
			a := (y + j + p.ImageHeight) % p.ImageHeight

			if world[b][a] == alive {
				count++
			}
		}
	}
	return count
}

func calculateAliveCells(p Params, world [][]byte) []cell {
	var arr []cell
	for x := 0; x < p.ImageWidth; x++ {
		for y := 0; y < p.ImageHeight; y++ {
			if world[x][y] == alive {
				arr = append(arr, cell{x: y, y: x})
			}
		}
	}
	return arr
}

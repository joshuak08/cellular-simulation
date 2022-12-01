package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/bStubs"
	"uk.ac.bris.cs/gameoflife/stubs"
)

const alive = 255
const dead = 0

// Global variables to interact with other RPC call functions
var mu sync.Mutex
var globalWorld [][]byte

// GoL logic to calculate next state for the world, that returns a 2d slice
func calculateNextState(world [][]byte, startY, endY, ImageHeight, ImageWidth int) [][]byte {
	// newGrid creates a new slice that will return the new world, with height that is proportionately separated with other nodes
	height := endY - startY
	newGrid := make([][]byte, height)
	for i := range newGrid {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, ImageWidth)
	}

	// It computes the GoL logic for its specific slice for each thread
	for i := startY; i < endY; i++ {
		for j := 0; j < ImageWidth; j++ {
			// Counts number of neighbours for each cell
			neighbours := countNeighbours(i, j, world, ImageHeight, ImageWidth)
			state := world[i][j]
			// Gol logic
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

// Counts the number of neighbours for each cell/entry
func countNeighbours(x, y int, world [][]byte, ImageHeight, ImageWidth int) int {
	var aliveCount = 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			// Don't count self as neighbour
			if i == 0 && j == 0 {
				continue
			}
			// Wrap-around. Add height and width for negative values
			r := (x + i + ImageWidth) % ImageWidth
			c := (y + j + ImageHeight) % ImageHeight
			if world[r][c] == alive {
				aliveCount++
			}
		}
	}
	return aliveCount
}

// GolOperations struct for broker to interact with server/worker nodes
type GolOperations struct{}

// RPC call from broker to server/nodes to calculate next state
func (s *GolOperations) CalculateNextWorld(req bStubs.Request, res *bStubs.Response) (err error) {
	globalWorld = req.World

	// globalWorld gets new world state
	mu.Lock()
	globalWorld = calculateNextState(req.World, req.StartY, req.EndY, req.Height, req.Width)
	mu.Unlock()

	// Updates response world with the global variables
	res.World = globalWorld
	return
}

// RPC call to shut down server
func (s *GolOperations) ShutServer(req stubs.Request, res *stubs.Response) (err error) {
	os.Exit(3)
	return
}

// Main function to setup the server and listens on port :8030
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	task := &GolOperations{}
	rpc.Register(task)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	rpc.Accept(listener)
}

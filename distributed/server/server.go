package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/bStubs"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const alive = 255
const dead = 0

var aliveCount int
var globalTurns int
var mu sync.Mutex
var globalWorld [][]byte
var shut chan bool

// Gol Logic

func calculateNextState(world [][]byte, startY, endY, ImageHeight, ImageWidth int) [][]byte {
	// Make allocates an array and returns a slice that refers to that array
	height := endY - startY
	newGrid := make([][]byte, height)
	for i := range newGrid {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, ImageWidth)
	}
	for i := startY; i < endY; i++ {
		for j := 0; j < ImageWidth; j++ {
			neighbours := countNeighbours(i, j, world, ImageHeight, ImageWidth)
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

func countNeighbours(x, y int, world [][]byte, ImageHeight, ImageWidth int) int {
	var aliveCount = 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			// Don't count self as neighbour
			if i == 0 && j == 0 {
				continue
			}
			// Wraparound. Add height and width for negative values
			r := (x + i + ImageWidth) % ImageWidth
			c := (y + j + ImageHeight) % ImageHeight
			if world[r][c] == alive {
				aliveCount++
			}
		}
	}
	return aliveCount
}

func calculateAliveCells(world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for i := 0; i < len(world); i++ {
		for j := 0; j < len(world[i]); j++ {
			if world[i][j] == alive {
				newCell := util.Cell{X: j, Y: i}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

type GolOperations struct {
	shut chan bool
}

func (s *GolOperations) CalculateNextWorld(req bStubs.Request, res *bStubs.Response) (err error) {
	turn := 0
	globalWorld = req.World
	//for turn < req.Turns {
	globalWorld = calculateNextState(req.World, req.StartY, req.EndY, req.Height, req.Width)
	lenAliveCount := len(calculateAliveCells(globalWorld))

	mu.Lock()
	aliveCount = lenAliveCount
	mu.Unlock()

	//fmt.Println("Alive Cells", aliveCount)
	turn++
	mu.Lock()
	globalTurns = turn
	mu.Unlock()
	//}
	res.Turns = turn
	res.World = globalWorld
	return
}

func (s *GolOperations) CalculateAlive(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.AliveCells = aliveCount
	mu.Unlock()

	mu.Lock()
	res.Turns = globalTurns
	mu.Unlock()
	return
}

func (s *GolOperations) ShutServer(req stubs.Request, res *stubs.Response) (err error) {
	os.Exit(3)
	//s.shut <- true
	//fmt.Println(req.Kill)
	//shut <- true
	//fmt.Println("test")
	//if req.Kill == true {
	//	os.Exit(3)
	//}
	return
}

func (s *GolOperations) Snapshot(req stubs.Request, res *stubs.Response) (err error) {
	res.Turns = globalTurns
	fmt.Println("globalWorld")
	//mu.Lock()
	res.World = globalWorld
	//mu.Unlock()
	return
}

func main() {
	//shut <- false
	//var port string
	//flag.StringVar(&port, "p", "root", "Specify username. Default is root")
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	task := &GolOperations{make(chan bool)}
	rpc.Register(task)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	//var mu sync.Mutex
	rpc.Accept(listener)
	//select {
	//case <-task.shut:
	//	fmt.Println("main")
	//	close(task.shut)
	//	listener.Close()
	//}
	//<-shut
	//			listener.Close()
	//			return
	//		}
	//	}
	//}()
}

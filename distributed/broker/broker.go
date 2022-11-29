package main

import (
	//"distributed/bStubs"
	//"bStubs"
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/bStubs"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const alive = 255
const dead = 0

var aliveCount int
var globalTurns int
var mu sync.Mutex
var globalWorld [][]byte
var workers []*rpc.Client

// Gol Logic

type distributorChannels struct {
	events chan<- gol.Event
	//ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

func makeCallWorld(client *rpc.Client, world [][]byte, ImageHeight, ImageWidth, StartY, EndY, Turns int, out chan [][]byte) {
	request := bStubs.Request{world, ImageWidth, StartY, EndY, ImageHeight, Turns, false}
	response := new(bStubs.Response)
	client.Call(bStubs.BTurnHandler, request, response)
	out <- response.World
	return
	//return response
}

//func makeCallAliveCells(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) *stubs.Response {
//	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false}
//	response := new(stubs.Response)
//	client.Call(stubs.AliveHandler, request, response)
//	return response
//}
//
//func makeCallSnapshot(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) *stubs.Response {
//	request := stubs.Request{world, ImageHeight, ImageWidth, Turns, false}
//	response := new(stubs.Response)
//	client.Call(stubs.SnapshotHandler, request, response)
//	return response
//}

func closeServers(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) {
	request := bStubs.Request{world, ImageWidth, 0, 0, ImageHeight, Turns, true}
	response := new(bStubs.Response)
	client.Call(bStubs.BShutHandler, request, response)
	return
}

func splitWorkers(req stubs.Request, world [][]byte, workers []*rpc.Client) [][]byte {

	//done := make(chan bool)
	//ticker := time.NewTicker(2 * time.Second)
	//block := make(chan bool)
	//pPressed := false

	//go func() {
	//	for {
	//		select {
	//		case key := <-keyPresses:
	//			snapshot := makeCallSnapshot(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
	//			switch key {
	//			case 's':
	//				c.ioCommand <- ioOutput
	//				outfile := strconv.Itoa(snapshot.Turns)
	//				c.ioFilename <- outfile
	//				for i := 0; i < p.ImageHeight; i++ {
	//					for j := 0; j < p.ImageWidth; j++ {
	//						c.ioOutput <- snapshot.World[i][j]
	//					}
	//				}
	//				c.events <- ImageOutputComplete{snapshot.Turns, outfile}
	//			case 'q':
	//				c.ioCommand <- ioOutput
	//				outfile := strconv.Itoa(snapshot.Turns)
	//				c.ioFilename <- outfile
	//				for i := 0; i < p.ImageHeight; i++ {
	//					for j := 0; j < p.ImageWidth; j++ {
	//						c.ioOutput <- world[i][j]
	//					}
	//				}
	//				fmt.Println("Quitting")
	//				c.events <- ImageOutputComplete{snapshot.Turns, outfile}
	//				c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
	//			case 'p':
	//				c.events <- StateChange{snapshot.Turns, Paused}
	//				pPressed = true
	//				for {
	//					switch <-keyPresses {
	//					case 'p':
	//						c.events <- StateChange{snapshot.Turns, Executing}
	//						fmt.Println("Continuing")
	//						pPressed = false
	//					}
	//					if !pPressed {
	//						break
	//					}
	//				}
	//			case 'k':
	//				c.ioCommand <- ioOutput
	//				outfile := strconv.Itoa(snapshot.Turns)
	//				c.ioFilename <- outfile
	//				for i := 0; i < p.ImageHeight; i++ {
	//					for j := 0; j < p.ImageWidth; j++ {
	//						c.ioOutput <- world[i][j]
	//					}
	//				}
	//				fmt.Println("Quitting and killing server")
	//				c.events <- ImageOutputComplete{snapshot.Turns, outfile}
	//				closeServer(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
	//				c.events <- FinalTurnComplete{snapshot.Turns, calculateAliveCells(p, snapshot.World)}
	//			}
	//		case <-done:
	//			return
	//			case <-ticker.C:
	//				response := makeCallAliveCells(client, world, p.ImageWidth, p.ImageHeight, p.Turns)
	//				cells := AliveCellsCount{response.Turns, response.AliveCells}
	//				c.events <- cells
	//		}
	//	}
	//}()
	out := make([]chan [][]byte, 4)
	for i := range out {
		out[i] = make(chan [][]byte)
	}

	for j := 0; j < 4; j++ {
		go makeCallWorld(workers[j], world, req.Height, req.Width, j*len(world)/4, (j+1)*(len(world))/4, req.Turns, out[j])
	}
	var newPixelData [][]byte
	for i := 0; i < len(out); i++ {
		newPixelData = append(newPixelData, <-out[i]...)
	}
	return newPixelData
}

type Broker struct {
	shut chan bool
}

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

func (s *Broker) CalculateNextWorld(req stubs.Request, res *stubs.Response) (err error) {
	turn := 0
	globalWorld = req.World
	for turn < req.Turns {
		globalWorld = splitWorkers(req, globalWorld, workers)
		lenAliveCount := len(calculateAliveCells(globalWorld))

		mu.Lock()
		aliveCount = lenAliveCount
		mu.Unlock()

		turn++
		mu.Lock()
		globalTurns = turn
		mu.Unlock()
	}
	res.Turns = turn
	res.World = globalWorld
	return
}

func (s *Broker) CalculateAlive(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.AliveCells = aliveCount
	mu.Unlock()

	mu.Lock()
	res.Turns = globalTurns
	mu.Unlock()
	return
}

func (s *Broker) ShutServer(req stubs.Request, res *stubs.Response) (err error) {
	for i := 0; i < 4; i++ {
		mu.Lock()
		closeServers(workers[i], req.World, req.Width, req.Height, req.Turns)
		mu.Unlock()
	}
	os.Exit(3)
	return
}

func (s *Broker) Snapshot(req stubs.Request, res *stubs.Response) (err error) {
	res.Turns = globalTurns
	//mu.Lock()
	res.World = globalWorld
	//mu.Unlock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	task := &Broker{make(chan bool)}
	rpc.Register(task)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	workers = make([]*rpc.Client, 4)
	address := "127.0.0.1:803"
	for i := 0; i < 4; i++ {
		end := strconv.Itoa(i + 1)
		workers[i], _ = rpc.Dial("tcp", address+end)
		//fmt.Println(address + end)
		//fmt.Println(reflect.TypeOf(address + end))
		defer workers[i].Close()
	}

	rpc.Accept(listener)
}

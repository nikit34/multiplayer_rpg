package backend

import (
	"sync"
	"time"
)


type Coordinate struct {
	X int
	Y int
}

type Direction int32

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
	DirectionStop
)

type Player struct {
	Position  Coordinate
	Name      string
	Direction Direction
	Icon      rune
	Mux       sync.Mutex
}

type Game struct {
	Players map[string]*Player
	Mux sync.Mutex
}

func NewGame() Game {
	game := Game{
		Players: make(map[string]*Player),
	}
	return game
}

func (game *Game) Start() {
	go func() {
		lastmove := map[string]time.Time{}
		for {
			game.Mux.Lock()
			for name, player := range game.Players {
				player.Mux.Lock()
				if player.Direction == DirectionStop || lastmove[name].After(time.Now().Add(-50 * time.Millisecond)) {
					player.Direction = DirectionStop
					player.Mux.Unlock()
					continue
				}
				switch player.Direction {
				case DirectionUp:
					player.Position.Y -= 1
				case DirectionDown:
					player.Position.Y += 1
				case DirectionLeft:
					player.Position.X -= 1
				case DirectionRight:
					player.Position.X += 1
				}
				player.Direction = DirectionStop
				lastmove[name] = time.Now()
				player.Mux.Unlock()
			}
			game.Mux.Unlock()
		}
	}()
}
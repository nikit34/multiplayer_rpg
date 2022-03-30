package backend

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)


type LaserChange struct {
	Change
	UUID uuid.UUID
	Laser Laser
}

type Coordinate struct {
	X int
	Y int
}

type PlayerState int

const (
	PlayerAlive PlayerState = iota
	PlayerDead
)

type Player struct {
	Position  Coordinate
	Name      string
	Icon      rune
	State     PlayerState
	Mux       sync.RWMutex
}

type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
	DirectionStop
)

type Laser struct {
	InitialPosition Coordinate
	Direction Direction
	StartTime time.Time
}

type Change interface{}

func (game *Game) GetPlayer(playerName string) *Player {
	game.Mux.RLock()
	defer game.Mux.RUnlock()
	player, ok := game.Players[playerName]
	if !ok {
		return nil
	}
	return player
}

func (game *Game) CheckLastActionTime(actionKey string, throttle int) bool {
	lastAction, ok := game.LastAction[actionKey]
	if ok && lastAction.After(time.Now().Add(-1*time.Duration(throttle)*time.Millisecond)) {
		return false
	}
	return true
}

func (game *Game) UpdateLastActionTime(actionKey string) {
	game.Mux.Lock()
	defer game.Mux.Unlock()
	game.LastAction[actionKey] = time.Now()
}

type MoveAction struct {
	Action
	PlayerName string
	Direction Direction
}

type PositionChange struct {
	Change
	PlayerName string
	Direction Direction
	Position Coordinate
}

func (action MoveAction) Perform(game *Game) {
	player := game.GetPlayer(action.PlayerName)
	if player == nil {
		return
	}
	actionKey := fmt.Sprintf("%T_%s", action, action.PlayerName)
	if !game.CheckLastActionTime(actionKey, 50) {
		return
	}

	player.Mux.Lock()

	switch action.Direction {
	case DirectionUp:
		player.Position.Y--
	case DirectionDown:
		player.Position.Y++
	case DirectionLeft:
		player.Position.X--
	case DirectionRight:
		player.Position.X++
	}

	game.LastAction[actionKey] = time.Now()

	change := PositionChange{
		PlayerName: player.Name,
		Direction: action.Direction,
		Position: player.Position,
	}

	defer player.Mux.Unlock()

	select {
	case game.ChangeChannel <- change:

	default:

	}

	game.UpdateLastActionTime(actionKey)
}

type LaserAction struct {
	Direction  Direction
	PlayerName string
}

func (action LaserAction) Perform(game *Game) {
	player := game.GetPlayer(action.PlayerName)
	if player == nil {
		return
	}
	actionKey := fmt.Sprintf("%T_%s", action, action.PlayerName)
	if !game.CheckLastActionTime(actionKey, 500) {
		return
	}

	player.Mux.RLock()

	laser := Laser{
		InitialPosition: player.Position,
		StartTime:       time.Now(),
		Direction:       action.Direction,
	}

	player.Mux.RUnlock()

	switch action.Direction {
	case DirectionUp:
		laser.InitialPosition.Y--
	case DirectionDown:
		laser.InitialPosition.Y++
	case DirectionLeft:
		laser.InitialPosition.X--
	case DirectionRight:
		laser.InitialPosition.X++
	}

	game.Mux.Lock()
	laserUUID := uuid.New()
	game.Lasers[laserUUID] = laser
	game.Mux.Unlock()

	change := LaserChange{
		Laser: laser,
		UUID: laserUUID,
	}

	select {
	case game.ChangeChannel <- change:

	default:

	}

	game.UpdateLastActionTime(actionKey)
}

type Action interface {
	Perform(game *Game)
}

type Game struct {
	Players map[string]*Player
	Lasers map[uuid.UUID]Laser
	Mux sync.RWMutex
	ChangeChannel chan Change
	ActionChannel chan Action
	LastAction map[string]time.Time
}

func NewGame() *Game {
	game := Game{
		Players: make(map[string]*Player),
		ActionChannel: make(chan Action, 1),
		LastAction: make(map[string]time.Time),
		ChangeChannel: make(chan Change, 1),
		Lasers: make(map[uuid.UUID]Laser),
	}
	return &game
}

type PlayerKilledChange struct {
	Change
	PlayerName string
	SpawnPosition Coordinate
}

type LaserRemoveChange struct {
	Change
	UUID uuid.UUID
}

func (game *Game) Start() {
	go func() {
		for {
			action := <-game.ActionChannel
			action.Perform(game)
		}
	}()

	go func() {
		for {
			game.Mux.Lock()
			for id, laser := range game.Lasers {
				laserPosition := laser.GetPosition()
				didCollide := false
				for _, player := range game.Players {
					player.Mux.Lock()

					if player.Position.X == laserPosition.X && player.Position.Y == laserPosition.Y {
						didCollide = true
						player.Position.X = 0
						player.Position.Y = 0
						change := PlayerKilledChange{
							PlayerName: player.Name,
							SpawnPosition: player.Position,
						}

						player.Mux.Unlock()
						select {
						case game.ChangeChannel <- change:

						default:

						}
					} else {
						player.Mux.Unlock()
					}
				}
				if didCollide {
					delete(game.Lasers, id)

					change:= LaserRemoveChange{
						UUID: id,
					}

					select {
					case game.ChangeChannel <- change:

					default:

					}
				}
			}
			game.Mux.Unlock()
			time.Sleep(time.Millisecond * 20)
		}
	}()
}

func (laser Laser) GetPosition() Coordinate {
	difference := time.Now().Sub(laser.StartTime)
	moves := int(math.Floor(float64(difference.Milliseconds()) / float64(20)))
	position := laser.InitialPosition
	switch laser.Direction {
	case DirectionUp:
		position.Y -= moves
	case DirectionDown:
		position.Y += moves
	case DirectionLeft:
		position.X -= moves
	case DirectionRight:
		position.X += moves
	}
	return position
}

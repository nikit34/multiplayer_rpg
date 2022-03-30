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
	ID uuid.UUID
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

func (e IdentifierBase) ID() uuid.UUID {
	return e.UUID
}

type Identifier interface {
	ID() uuid.UUID
}

type IdentifierBase struct {
	UUID uuid.UUID
}

type Positioner interface {
	Position() Coordinate
}

type Mover interface {
	Move(Coordinate)
}

type Player struct {
	IdentifierBase
	Positioner
	Mover
	position Coordinate
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
	IdentifierBase
	Positioner
	InitialPosition Coordinate
	Direction Direction
	StartTime time.Time
}

type Change interface{}

func (game *Game) AddEntity(entity Identifier) {
	game.Mu.Lock()
	game.Entities[entity.ID()] = entity
	game.Mu.Unlock()
}

func (game *Game) GetEntity(id uuid.UUID) Identifier {
	game.Mu.RLock()
	defer game.Mu.RUnlock()
	return game.Entities[id]
}

func (game *Game) RemoveEntity(id uuid.UUID) {
	game.Mu.Lock()
	delete(game.Entities, id)
	game.Mu.Unlock()
}

func (game *Game) CheckLastActionTime(actionKey string, throttle int) bool {
	lastAction, ok := game.LastAction[actionKey]
	if ok && lastAction.After(time.Now().Add(-1*time.Duration(throttle)*time.Millisecond)) {
		return false
	}
	return true
}

func (game *Game) UpdateLastActionTime(actionKey string) {
	game.Mu.Lock()
	game.LastAction[actionKey] = time.Now()
	game.Mu.Unlock()
}

type MoveAction struct {
	ID uuid.UUID
	Direction Direction
}

type PositionChange struct {
	Change
	ID uuid.UUID
	Direction Direction
	Position Coordinate
}

func (action MoveAction) Perform(game *Game) {
	entity := game.GetEntity(action.ID)
	if entity == nil {
		return
	}

	actionKey := fmt.Sprintf("%T:%s", action, entity.ID().String())
	if !game.CheckLastActionTime(actionKey, 50) {
		return
	}

	position := entity.(Positioner).Position()

	switch action.Direction {
	case DirectionUp:
		position.Y--
	case DirectionDown:
		position.Y++
	case DirectionLeft:
		position.X--
	case DirectionRight:
		position.X++
	}
	entity.(Mover).Move(position)


	change := PositionChange{
		ID: entity.ID(),
		Direction: action.Direction,
		Position: position,
	}

	select {
	case game.ChangeChannel <- change:

	default:

	}

	game.UpdateLastActionTime(actionKey)
}

type LaserAction struct {
	Direction  Direction
	OwnerID    uuid.UUID
}

func (action LaserAction) Perform(game *Game) {
	entity := game.GetEntity(action.OwnerID)
	if entity == nil {
		return
	}

	actionKey := fmt.Sprintf("%T:%s", action, entity.ID().String())
	if !game.CheckLastActionTime(actionKey, 500) {
		return
	}

	laser := Laser{
		InitialPosition: entity.(Positioner).Position(),
		StartTime:       time.Now(),
		Direction:       action.Direction,
		IdentifierBase:  IdentifierBase{uuid.New()},
	}

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

	game.AddEntity(&laser)

	change := LaserChange{
		Laser: laser,
		ID: laser.ID(),
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
	Entities map[uuid.UUID]Identifier
	Mu sync.RWMutex
	ChangeChannel chan Change
	ActionChannel chan Action
	LastAction map[string]time.Time
}

func NewGame() *Game {
	game := Game{
		Entities: make(map[uuid.UUID]Identifier),
		ActionChannel: make(chan Action, 1),
		LastAction: make(map[string]time.Time),
		ChangeChannel: make(chan Change, 1),
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
	ID uuid.UUID
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
			// game.Mux.Lock()
			// for id, laser := range game.Lasers {
			// 	laserPosition := laser.GetPosition()
			// 	didCollide := false
			// 	for _, player := range game.Players {
			// 		player.Mux.Lock()

			// 		if player.Position.X == laserPosition.X && player.Position.Y == laserPosition.Y {
			// 			didCollide = true
			// 			player.Position.X = 0
			// 			player.Position.Y = 0
			// 			change := PlayerKilledChange{
			// 				PlayerName: player.Name,
			// 				SpawnPosition: player.Position,
			// 			}

			// 			player.Mux.Unlock()
			// 			select {
			// 			case game.ChangeChannel <- change:

			// 			default:

			// 			}
			// 		} else {
			// 			player.Mux.Unlock()
			// 		}
			// 	}
			// 	if didCollide {
			// 		delete(game.Lasers, id)

			// 		change:= LaserRemoveChange{
			// 			UUID: id,
			// 		}

			// 		select {
			// 		case game.ChangeChannel <- change:

			// 		default:

			// 		}
			// 	}
			// }
			// game.Mux.Unlock()
			time.Sleep(time.Millisecond * 20)
		}
	}()
}

func (laser *Laser) Position() Coordinate {
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

func (p *Player) Position() Coordinate {
	return p.position
}

func (p *Player) Move(c Coordinate) {
	p.position = c
}

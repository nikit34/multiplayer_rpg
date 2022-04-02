package backend

import (
	"fmt"
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


type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
	DirectionStop
)

type Change interface{}

func (game *Game) AddEntity(entity Identifier) {
	game.Mu.Lock()
	game.Entities[entity.ID()] = entity
	game.Mu.Unlock()
}

func (game *Game) UpdateEntity(entity Identifier) {
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
	Entity Identifier
	Direction Direction
	Position Coordinate
}

type AddEntityChange struct {
	Change
	Entity Identifier
}

type RemoveEntityChange struct {
	Change
	Entity Identifier
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
		Entity: entity,
		Direction: action.Direction,
		Position: position,
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

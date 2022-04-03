package backend

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type LaserChange struct {
	Change
	ID    uuid.UUID
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
	game.Mu.RLock()
	lastAction, ok := game.LastAction[actionKey]
	game.Mu.RUnlock()

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
	ID        uuid.UUID
	Direction Direction
}

type MoveChange struct {
	Change
	Entity    Identifier
	Direction Direction
	Position  Coordinate
}

type AddEntityChange struct {
	Change
	Entity Identifier
}

type RemoveEntityChange struct {
	Change
	Entity Identifier
}

func (game *Game) Move(id uuid.UUID, position Coordinate) {
	game.Mu.Lock()
	game.Entities[id].(Mover).Move(position)
	game.Mu.Unlock()
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
	game.Move(entity.ID(), position)

	change := MoveChange{
		Entity:    entity,
		Direction: action.Direction,
		Position:  position,
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
	Entities        map[uuid.UUID]Identifier
	Mu              sync.RWMutex
	ChangeChannel   chan Change
	ActionChannel   chan Action
	LastAction      map[string]time.Time
	IsAuthoritative bool
	Score map[uuid.UUID]int
}

func NewGame() *Game {
	game := Game{
		Entities:        make(map[uuid.UUID]Identifier),
		ActionChannel:   make(chan Action, 1),
		LastAction:      make(map[string]time.Time),
		ChangeChannel:   make(chan Change, 1),
		IsAuthoritative: true,
		Score: make(map[uuid.UUID]int),
	}
	return &game
}

type LaserRemoveChange struct {
	Change
	ID uuid.UUID
}

type PlayerRespawnChange struct {
	Change
	Player *Player
}

func (game *Game) Start() {
	go func() {
		for {
			action := <-game.ActionChannel
			action.Perform(game)
		}
	}()

	if !game.IsAuthoritative {
		return
	}

	go func() {
		for {
			collisionMap := make(map[Coordinate][]Positioner)

			game.Mu.RLock()
			for _, entity := range game.Entities {
				positioner, ok := entity.(Positioner)
				if !ok {
					continue
				}

				position := positioner.Position()
				collisionMap[position] = append(collisionMap[position], positioner)
			}
			game.Mu.RUnlock()

			for _, entities := range collisionMap {
				if len(entities) <= 1 {
					continue
				}

				hasLaser := false

				var laserOwnerID uuid.UUID
				for _, entity := range entities {
					laser, ok := entity.(*Laser)
					if ok {
						hasLaser = true
						laserOwnerID = laser.OwnerID
						break
					}
				}

				if hasLaser {
					for _, entity := range entities {
						switch type_entity := entity.(type) {
						case *Laser:
							laser := type_entity
							change := RemoveEntityChange{
								Entity: laser,
							}

							select {
							case game.ChangeChannel <- change:

							default:

							}

							game.RemoveEntity(laser.ID())
						case *Player:
							player := type_entity

							game.Move(player.ID(), Coordinate{X: 0, Y: 0})

							change := PlayerRespawnChange{
								Player: player,
							}

							select {
							case game.ChangeChannel <- change:

							default:

							}

							if player.ID() != laserOwnerID {
								game.Mu.Lock()
								game.Score[laserOwnerID]++
								game.Mu.Unlock()
							}
						}
					}
				}
			}

			time.Sleep(time.Millisecond * 20)
		}
	}()
}

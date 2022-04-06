package backend

import (
	"fmt"
	"sync"
	"time"
	"math"

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
	game.Entities[entity.ID()] = entity
}

func (game *Game) UpdateEntity(entity Identifier) {
	game.Entities[entity.ID()] = entity
}

func (game *Game) GetEntity(id uuid.UUID) Identifier {
	return game.Entities[id]
}

func (game *Game) RemoveEntity(id uuid.UUID) {
	delete(game.Entities, id)
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
	game.LastAction[actionKey] = time.Now()
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

func (game *Game) GetMapSymbols() map[rune][]Coordinate {
	mapCenterX := len(game.Map[0]) / 2
	mapCenterY := len(game.Map) / 2
	symbols := make(map[rune][]Coordinate, 0)
	for mapY, row := range game.Map {
		for mapX, col := range row {
			if col == ' ' {
				continue
			}
			symbols[col] = append(symbols[col], Coordinate{
				X: mapX - mapCenterX,
				Y: mapY - mapCenterY,
			})
		}
	}
	return symbols
}

func (game *Game) Move(id uuid.UUID, position Coordinate) {
	game.Entities[id].(Mover).Move(position)
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

	for _, wall := range game.GetMapWalls() {
		if position == wall {
			return
		}
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
	WaitForRound bool
	RoundWinner uuid.UUID
	NewRoundAt time.Time
	Map [][]rune
}

func NewGame() *Game {
	game := Game{
		Entities:        make(map[uuid.UUID]Identifier),
		ActionChannel:   make(chan Action, 1),
		LastAction:      make(map[string]time.Time),
		ChangeChannel:   make(chan Change, 1),
		IsAuthoritative: true,
		WaitForRound: false,
		Score: make(map[uuid.UUID]int),
		Map: MapDefault,
	}
	return &game
}

type LaserRemoveChange struct {
	Change
	ID uuid.UUID
}

func (game *Game) GetMapWalls() []Coordinate {
	return game.GetMapSymbols()['█']
}

func (game *Game) GetMapSpawnPoints() []Coordinate {
	return game.GetMapSymbols()['S']
}

func Distance(a Coordinate, b Coordinate) int {
	return int(math.Sqrt(math.Pow(float64(b.X-a.X), 2) + math.Pow(float64(b.Y-a.Y), 2)))
}

type PlayerRespawnChange struct {
	Change
	Player *Player
}

func (game *Game) AddScore(id uuid.UUID) {
	game.Score[id]++
	if game.Score[id] >= 10 {
		game.Score = make(map[uuid.UUID]int)
		game.WaitForRound = true
		game.NewRoundAt = time.Now().Add(time.Second * 10)
		game.RoundWinner = id

		go func() {
			time.Sleep(time.Second * 10)
			game.Mu.Lock()
			game.WaitForRound = false
			game.Mu.Unlock()
		}()
	}
}

func (game *Game) Start() {
	go func() {
		for {
			action := <-game.ActionChannel
			if game.WaitForRound {
				continue
			}

			game.Mu.Lock()
			action.Perform(game)
			game.Mu.Unlock()
		}
	}()

	if !game.IsAuthoritative {
		return
	}

	go func() {
		for {
			game.Mu.Lock()
			collisionMap := make(map[Coordinate][]Identifier)

			game.Mu.RLock()
			for _, entity := range game.Entities {
				positioner, ok := entity.(Positioner)
				if !ok {
					continue
				}

				position := positioner.Position()
				collisionMap[position] = append(collisionMap[position], entity)
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

				if !hasLaser {
					continue
				}

				for _, entity := range entities {
					switch type_entity := entity.(type) {
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
							game.AddScore(laserOwnerID)
						}

					default:
						change := RemoveEntityChange{
							Entity: entity,
						}

						select {
						case game.ChangeChannel <- change:

						default:

						}
						game.RemoveEntity(entity.ID())
					}
				}
			}

			for _, wall := range game.GetMapWalls() {
				entities, ok := collisionMap[wall]
				if !ok {
					continue
				}
				for _, entity := range entities {
					switch entity.(type) {
					case *Laser:
						change := RemoveEntityChange{
							Entity: entity,
						}
						select {
						case game.ChangeChannel <- change:
						default:
						}
						game.RemoveEntity(entity.ID())
					}
				}
			}

			game.Mu.Unlock()
			time.Sleep(time.Millisecond * 20)
		}
	}()
}

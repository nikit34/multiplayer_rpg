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

type PlayerRespawnChange struct {
	Change
	Player     *Player
	KilledByID uuid.UUID
}

func (game *Game) SendChange(change Change) {
	select {
	case game.ChangeChannel <- change:

	default:

	}
}

func (action MoveAction) Perform(game *Game) {
	entity := game.GetEntity(action.ID)
	if entity == nil {
		return
	}

	mover, ok := entity.(Mover)
	if !ok {
		return
	}

	positioner, ok := entity.(Positioner)
	if !ok {
		return
	}

	actionKey := fmt.Sprintf("%T:%s", action, entity.ID().String())
	if !game.CheckLastActionTime(actionKey, 50) {
		return
	}

	position := positioner.Position()

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

	mover.Move(position)

	change := MoveChange{
		Entity:    entity,
		Direction: action.Direction,
		Position:  position,
	}

	game.SendChange(change)
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
	Score           map[uuid.UUID]int
	WaitForRound    bool
	RoundWinner     uuid.UUID
	NewRoundAt      time.Time
	Map             [][]rune
}

func NewGame() *Game {
	game := Game{
		Entities:        make(map[uuid.UUID]Identifier),
		ActionChannel:   make(chan Action, 1),
		LastAction:      make(map[string]time.Time),
		ChangeChannel:   make(chan Change, 1),
		IsAuthoritative: true,
		WaitForRound:    false,
		Score:           make(map[uuid.UUID]int),
		Map:             MapDefault,
	}
	return &game
}

type LaserRemoveChange struct {
	Change
	ID uuid.UUID
}

func Distance(a Coordinate, b Coordinate) int {
	return int(math.Sqrt(math.Pow(float64(b.X-a.X), 2) + math.Pow(float64(b.Y-a.Y), 2)))
}

type RoundOverChange struct {
	Change
}

type RoundStartChange struct {
	Change
}

func (game *Game) StartNewRound(roundWinner uuid.UUID) {
	game.Score = make(map[uuid.UUID]int)
		game.WaitForRound = true
		game.NewRoundAt = time.Now().Add(time.Second * 10)
		game.RoundWinner = roundWinner

		game.SendChange(RoundOverChange{})

		go func() {
			time.Sleep(time.Second * 10)
			game.Mu.Lock()
			game.WaitForRound = false

			i := 0
			spawnPoints := game.GetMapSpawnPoints()
			for _, entity := range game.Entities {
				player, ok := entity.(*Player)
				if !ok {
					continue
				}
				player.CurrentPosition = spawnPoints[i%len(spawnPoints)]
				i++
			}

			game.Mu.Unlock()
			game.SendChange(RoundStartChange{})
		}()
}

func (game *Game) AddScore(id uuid.UUID) {
	game.Score[id]++
	if game.Score[id] >= 10 {
		game.StartNewRound(id)
	}
}

func (game *Game) watchActions() {
	for {
		action := <-game.ActionChannel
		if game.WaitForRound {
			continue
		}

		game.Mu.Lock()
		action.Perform(game)
		game.Mu.Unlock()
	}
}

func (game *Game) getCollisionMap() map[Coordinate][]Identifier {
	collisionMap := map[Coordinate][]Identifier{}
	for _, entity := range game.Entities {
		positioner, ok := entity.(Positioner)
		if !ok {
			continue
		}

		position := positioner.Position()
		collisionMap[position] = append(collisionMap[position], entity)
	}
	return collisionMap
}

func (game *Game) watchCollisions() {
	for {
		game.Mu.Lock()

		collisionMap := game.getCollisionMap()

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
					if !game.IsAuthoritative {
						continue
					}

					player := type_entity
					if player.ID() == laserOwnerID {
						continue
					}

					spawnPoints := game.GetMapSpawnPoints()
					spawnPoint := spawnPoints[0]
					for _, sp := range game.GetMapSpawnPoints() {
						if Distance(player.Position(), sp) > Distance(player.Position(), spawnPoint) {
							spawnPoint = sp
						}
					}
					player.Move(spawnPoint)

					change := PlayerRespawnChange{
						Player:     player,
						KilledByID: laserOwnerID,
					}

					game.SendChange(change)
					game.AddScore(laserOwnerID)

				case *Laser:
					change := RemoveEntityChange{
						Entity: entity,
					}

					game.SendChange(change)
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

					game.SendChange(change)
					game.RemoveEntity(entity.ID())
				}
			}
		}

		game.Mu.Unlock()
		time.Sleep(time.Millisecond * 20)
	}
}

func (game *Game) Start() {
	go game.watchActions()
	go game.watchCollisions()
}

package backend

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Laser struct {
	IdentifierBase
	Positioner
	InitialPosition Coordinate
	Direction       Direction
	OwnerID         uuid.UUID
	StartTime       time.Time
}

func (laser *Laser) Position() Coordinate {
	difference := time.Since(laser.StartTime)
	moves := int(math.Floor(float64(difference.Milliseconds()) / float64(21)))
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

type LaserAction struct {
	Direction Direction
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Created time.Time
}

func (action LaserAction) Perform(game *Game) {
	entity := game.GetEntity(action.OwnerID)
	if entity == nil {
		return
	}

	actionKey := fmt.Sprintf("%T:%s", action, entity.ID().String())
	if !game.checkLastActionTime(actionKey, action.Created, laserThrottle) {
		return
	}

	laser := Laser{
		InitialPosition: entity.(Positioner).Position(),
		StartTime:       action.Created,
		Direction:       action.Direction,
		OwnerID:         action.OwnerID,
		IdentifierBase:  IdentifierBase{action.ID},
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

	change := AddEntityChange{Entity: &laser}

	game.sendChange(change)
	game.updateLastActionTime(actionKey, action.Created)
}

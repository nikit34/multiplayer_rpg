package proto

import (
	"log"
	"unicode/utf8"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg/pkg/backend"
)

func GetBackendDirection(protoDirection Direction) backend.Direction {
	direction := backend.DirectionStop
	switch protoDirection {
	case Direction_UP:
		direction = backend.DirectionUp
	case Direction_DOWN:
		direction = backend.DirectionDown
	case Direction_LEFT:
		direction = backend.DirectionLeft
	case Direction_RIGHT:
		direction = backend.DirectionRight
	}
	return direction
}

func GetProtoDirection(direction backend.Direction) Direction {
	protoDirection := Direction_STOP
	switch direction {
	case backend.DirectionUp:
		protoDirection = Direction_UP
	case backend.DirectionDown:
		protoDirection = Direction_DOWN
	case backend.DirectionLeft:
		protoDirection = Direction_LEFT
	case backend.DirectionRight:
		protoDirection = Direction_RIGHT
	}
	return protoDirection
}

func GetBackendCoordinate(protoCoordinate *Coordinate) backend.Coordinate {
	return backend.Coordinate{
		X: int(protoCoordinate.X),
		Y: int(protoCoordinate.Y),
	}
}

func GetProtoCoordinate(coordinate backend.Coordinate) *Coordinate {
	return &Coordinate{
		X: int32(coordinate.X),
		Y: int32(coordinate.Y),
	}
}

func GetBackendPlayer(protoPlayer *Player) *backend.Player {
	entityID, err := uuid.Parse(protoPlayer.Id)
	if err != nil {
		return nil
	}

	icon, _ := utf8.DecodeRuneInString(protoPlayer.Icon)
	player := &backend.Player{
		IdentifierBase: backend.IdentifierBase{UUID: entityID},
		Name:           protoPlayer.Name,
		Icon:           icon,
	}
	player.Move(GetBackendCoordinate(protoPlayer.Position))
	return player
}

func GetBackendLaser(protoLaser *Laser) *backend.Laser {
	entityID, err := uuid.Parse(protoLaser.Id)
	if err != nil {
		return nil
	}
	ownerID, err := uuid.Parse(protoLaser.OwnerId)
	if err != nil {
		return nil
	}
	timestamp, err := ptypes.Timestamp(protoLaser.StartTime)
	if err != nil {
		return nil
	}
	laser := &backend.Laser{
		IdentifierBase:  backend.IdentifierBase{UUID: entityID},
		InitialPosition: GetBackendCoordinate(protoLaser.InitialPosition),
		Direction:       GetBackendDirection(protoLaser.Direction),
		StartTime:       timestamp,
		OwnerID:         ownerID,
	}
	return laser
}

func GetBackendEntity(protoEntity *Entity) backend.Identifier {
	switch proto_type := protoEntity.Entity.(type) {
	case *Entity_Player:
		protoPlayer := proto_type.Player
		return GetBackendPlayer(protoPlayer)
	case *Entity_Laser:
		protoLaser := proto_type.Laser
		return GetBackendLaser(protoLaser)
	}
	log.Fatalf("Cannot get backend entity for %T -> %+v", protoEntity, protoEntity)
	return nil
}

func GetProtoPlayer(player *backend.Player) *Player {
	return &Player{
		Id:       player.ID().String(),
		Name:     player.Name,
		Position: GetProtoCoordinate(player.Position()),
		Icon: string(player.Icon),
	}
}

func GetProtoLaser(laser *backend.Laser) *Laser {
	timestamp, err := ptypes.TimestampProto(laser.StartTime)
	if err != nil {
		return nil
	}
	return &Laser{
		Id:              laser.ID().String(),
		StartTime:       timestamp,
		InitialPosition: GetProtoCoordinate(laser.InitialPosition),
		Direction:       GetProtoDirection(laser.Direction),
		OwnerId:         laser.OwnerID.String(),
	}
}

func GetProtoEntity(entity backend.Identifier) *Entity {
	switch entity_type := entity.(type) {
	case *backend.Player:
		protoPlayer := Entity_Player{
			Player: GetProtoPlayer(entity_type),
		}
		return &Entity{Entity: &protoPlayer}
	case *backend.Laser:
		protoLaser := Entity_Laser{
			Laser: GetProtoLaser(entity_type),
		}
		return &Entity{Entity: &protoLaser}
	}
	log.Fatalf("Cannot get proto entity for %T -> %+v", entity, entity)
	return nil
}

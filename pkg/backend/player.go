package backend

import "github.com/google/uuid"


type Player struct {
	IdentifierBase
	Positioner
	Mover
	position Coordinate
	Name     string
	Icon     rune
}

func (p *Player) Position() Coordinate {
	return p.position
}

func (p *Player) Move(c Coordinate) {
	p.position = c
}

type PlayerKilledChange struct {
	Change
	ID            uuid.UUID
	SpawnPosition Coordinate
}

package backend


type Player struct {
	IdentifierBase
	Positioner
	Mover
	CurrentPosition Coordinate
	Name     string
	Icon     rune
}

func (p *Player) Position() Coordinate {
	return p.CurrentPosition
}

func (p *Player) Move(c Coordinate) {
	p.CurrentPosition = c
}

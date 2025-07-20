package main

import (
	"log"

	"github.com/anthdm/hollywood/actor"
)

type MatchJoined struct {
	PID *actor.PID
}

type Match struct {
	p1 *actor.PID
	p2 *actor.PID
}

func NewMatch(p1, p2 *actor.PID) actor.Producer {
	return func() actor.Receiver {
		return &Match{p1: p1, p2: p2}
	}
}

func (m *Match) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		log.Printf("Match iniziato tra %s e %s", m.p1.String(), m.p2.String())
		// Dico ai giocatori che sono collegati al match
		c.Send(m.p1, &MatchJoined{PID: c.PID()})
		c.Send(m.p2, &MatchJoined{PID: c.PID()})

	case *PlayerAction:
		var recipient *actor.PID
		if msg.From.String() == m.p1.String() {
			recipient = m.p2
		} else {
			recipient = m.p1
		}
		c.Send(recipient, msg)
	}
}

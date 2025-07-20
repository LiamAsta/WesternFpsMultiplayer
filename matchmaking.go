package main

import (
	"fmt"
	"time"

	"github.com/anthdm/hollywood/actor"
)

type Matchmaking struct {
	players map[string]*PlayerStatus
}

func NewMatchmaking() actor.Producer {
	return func() actor.Receiver {
		return &Matchmaking{players: make(map[string]*PlayerStatus)}
	}
}

func (m *Matchmaking) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {

	case actor.Started:
		go m.MatchLoop(c)

	case *actor.PID:
		// Aggiunta giocatore
		m.players[msg.String()] = &PlayerStatus{PID: msg, Free: true}
		fmt.Printf("Giocatore %s aggiunto al matchmaking\n", msg.String())
	}
}

func (m *Matchmaking) MatchLoop(c *actor.Context) {
	for {
		time.Sleep(1 * time.Second) // intervallo di controllo matchmaking
		var p1, p2 *PlayerStatus

		// Trova due giocatori liberi
		for _, player := range m.players {
			if player.Free {
				if p1 == nil {
					p1 = player
					continue
				}
				p2 = player
				break
			}
		}

		if p1 != nil && p2 != nil {

			p1.Free = false
			p2.Free = false

			fmt.Printf(" Match trovato: %s vs %s\n", p1.PID.String(), p2.PID.String())
			c.SpawnChild(NewMatch(p1.PID, p2.PID), "match")
		}
	}
}

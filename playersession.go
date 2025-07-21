package main

import (
	"encoding/json"
	"log"

	"github.com/anthdm/hollywood/actor"
	"github.com/gorilla/websocket"
)

type PlayerSession struct {
	conn        *websocket.Conn
	matchmaking *actor.PID
	sessionPID  *actor.PID
	matchPID    *actor.PID
}

func NewSession(conn *websocket.Conn) actor.Producer {
	return func() actor.Receiver {
		return &PlayerSession{conn: conn}
	}
}

func (ps *PlayerSession) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		ps.sessionPID = c.PID()
		// Avvia la lettura dei messaggi WS
		go ps.readLoop(c)

	case *actor.PID:
		// msg è il PID del matchmaking
		ps.matchmaking = msg
		// Registrati al matchmaking
		c.Send(ps.matchmaking, ps.sessionPID)
		log.Println("Registrato al matchmaking:", ps.sessionPID.String())
	case *PlayerAction:
		// Gira il messaggio WebSocket al client Unity
		ps.conn.WriteMessage(websocket.TextMessage, []byte(msg.Data))
	case *MatchJoined:
		ps.matchPID = msg.PID
		log.Println("Assegnato al match:", ps.matchPID.String())

	}

}

func (ps *PlayerSession) readLoop(c *actor.Context) {
	for {
		_, data, err := ps.conn.ReadMessage()
		if err != nil {
			log.Println("WS read error:", err)
			c.Engine().Poison(ps.sessionPID) // termina l’attore
			return
		}

		var m struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			log.Println("JSON Unmarshal error:", err, "Data:", string(data))
			continue
		}

		log.Printf("Ricevuto action: %s dal player: %s", m.Action, ps.sessionPID.String())

		switch m.Action {
		case "login":
			log.Println("Login ricevuto")
		case "shoot":
			if ps.matchPID != nil {
				log.Printf("Inoltro action: %s al match %s", m.Action, ps.matchPID.String())
				c.Send(ps.matchPID, &PlayerAction{
					From:   ps.sessionPID,
					Action: m.Action,
					Data:   string(data),
				})
			}
		case "move":
			if ps.matchPID != nil {
				c.Send(ps.matchPID, &PlayerAction{
					From:   ps.sessionPID,
					Action: m.Action,
					Data:   string(data),
				})
			}

		default:
			log.Printf("Azione non gestita: %s", m.Action)
		}

	}
}

type PlayerAction struct {
	From   *actor.PID
	Action string
	Data   string
}

// Stato giocatore
type PlayerStatus struct {
	PID  *actor.PID
	Free bool
}

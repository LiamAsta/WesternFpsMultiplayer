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
		go ps.readLoop(c)

	case *actor.PID:
		// Ricevi il PID del matchmaking e registrati
		ps.matchmaking = msg
		c.Send(ps.matchmaking, ps.sessionPID)
		log.Println("Registrato al matchmaking:", ps.sessionPID.String())

	case *PlayerAction:
		// Messaggio da inoltrare al client Unity
		ps.conn.WriteMessage(websocket.TextMessage, []byte(msg.Data))

	}
}

func (ps *PlayerSession) readLoop(c *actor.Context) {
	for {
		_, data, err := ps.conn.ReadMessage()
		if err != nil {
			log.Println("WS read error:", err)
			c.Engine().Poison(ps.sessionPID)
			return
		}

		var m struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			log.Println("JSON Unmarshal error:", err, "Data:", string(data))
			// In caso di JSON errato, chiudi la connessione per evitare problemi di protocollo
			c.Engine().Poison(ps.sessionPID)
			return
		}

		log.Printf("Ricevuto action: %s dal player: %s", m.Action, ps.sessionPID.String())

		if ps.matchPID == nil {
			log.Println("Nessun match assegnato, ignoro action:", m.Action)
			continue
		}

		// Supporta tutte le azioni di gameplay
		switch m.Action {
		case "login", "shoot", "move", "buy_weapon", "plant_bomb", "defuse_bomb", "explosion_damage":
			c.Send(ps.matchPID, &PlayerAction{
				From:   ps.sessionPID,
				Action: m.Action,
				Data:   string(data),
			})
		default:
			log.Printf("Azione non gestita dal server: %s", m.Action)
		}
	}
}

type PlayerAction struct {
	From   *actor.PID
	Action string
	Data   string
}

// Stato giocatore per matchmaking
type PlayerStatus struct {
	PID  *actor.PID
	Free bool
}

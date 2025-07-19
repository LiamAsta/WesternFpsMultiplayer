package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
		ps.matchmaking = msg
		// Registro al matchmaking
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
			log.Println("JSON Unmarshal:", err)
			continue
		}

		switch m.Action {
		case "login":
			
		case "move", "shoot":
			
			if ps.matchPID != nil {
				c.Send(ps.matchPID, &PlayerAction{
					From:   ps.sessionPID,
					Action: m.Action,
					Data:   string(data), // inoltra l’intero JSON ricevuto
				})
			}
		}
	}
}

// Stato giocatore
type PlayerStatus struct {
	PID  *actor.PID
	Free bool
}
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
		time.Sleep(1 * time.Second) 
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

type Server struct {
	address        string
	matchmakingPID *actor.PID
}

func NewServer(addr string) actor.Producer {
	return func() actor.Receiver {
		return &Server{address: addr}
	}
}

func (s *Server) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:

		s.startHTTP(c)

		s.matchmakingPID = c.SpawnChild(NewMatchmaking(), "matchmaking")

	case *actor.PID:
		c.Send(s.matchmakingPID, msg)
	}
}
func (s *Server) startHTTP(c *actor.Context) {
	go func() {
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			// Upgrade a WebSocket
			conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
			if err != nil {
				log.Println("WS Upgrade:", err)
				return
			}
			// Spawn PlayerSession 
			sessionPID := c.SpawnChild(NewSession(conn), "session")
			c.Send(sessionPID, s.matchmakingPID)
		})
		log.Println("Server WS in ascolto su :4000/ws")
		http.ListenAndServe(":4000", nil)
	}()
}

type PlayerAction struct {
	From   *actor.PID
	Action string
	Data   string
}

func main() {
	e, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		panic(err)
	}

	e.Spawn(NewServer(":4000"), "server")
	select {}
}

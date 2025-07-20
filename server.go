package main

import (
	"log"
	"net/http"

	"github.com/anthdm/hollywood/actor"
	"github.com/gorilla/websocket"
)

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
			// Spawn PlayerSession e invia PID matchmaking
			sessionPID := c.SpawnChild(NewSession(conn), "session")
			// Comunica al session actor anche il PID del matchmaking
			c.Send(sessionPID, s.matchmakingPID)
		})
		log.Println("Server WS in ascolto su :4000/ws")
		http.ListenAndServe(":4000", nil)
	}()
}

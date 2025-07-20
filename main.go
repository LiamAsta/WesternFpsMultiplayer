package main

import (
	"github.com/anthdm/hollywood/actor"
)

func main() {
	e, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		panic(err)
	}

	e.Spawn(NewServer(":4000"), "server") //->
	select {}
}

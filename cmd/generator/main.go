package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jlpedrosa/distributed-scheduler/internal/ticker"
)

const nClients = 1000 * 1000
const httpAPIURl = "http://scheduler:8888/tick"

func main() {

	// generate a bunch of "clients"
	clients := make([]string, 0, nClients)
	for i := 0; i < nClients; i++ {
		clients = append(clients, uuid.NewString())
	}

	generator := loadGenerator{clients: clients}
	generator.GenerateLoad()
}

type loadGenerator struct {
	clients []string
}

func (l *loadGenerator) GenerateLoad() {
	for {
		ticker := l.fakeTicker()
		l.CreateTick(ticker)
		time.Sleep(time.Second)
	}
}

func (l *loadGenerator) CreateTick(tick ticker.TickDTO) {
	buffer, err := json.Marshal(tick)
	if err != nil {
		log.Fatalf("error marshalling ticker %v", err)
	}
	_, err = http.Post(httpAPIURl, "application/json", bytes.NewReader(buffer))
	if err != nil {
		log.Fatalf("error sending request %v", err)
	}
}

// fakeTicker creates a new ticker (simulating some API)
func (l *loadGenerator) fakeTicker() ticker.TickDTO {
	chosenOne := l.clients[rand.Intn(len(l.clients))]
	futureTime := time.Duration(rand.Intn(600)) * time.Second
	return ticker.TickDTO{
		Item: chosenOne,
		At:   time.Now().Add(futureTime),
	}

}

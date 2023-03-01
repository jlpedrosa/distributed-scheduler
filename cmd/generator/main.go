package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/jlpedrosa/distributed-scheduler/internal/ticker"
)

const nClients = 1000 * 1000
const httpAPIURl = "http://scheduler:8888/tick"
const nWorkers = 100

var ticksGenerated = atomic.Int32{}

func main() {

	// generate a bunch of "clients"
	clients := make([]string, 0, nClients)
	for i := 0; i < nClients; i++ {
		clients = append(clients, uuid.NewString())
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxConnsPerHost:       100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableKeepAlives:     false,
			ForceAttemptHTTP2:     true,
			DisableCompression:    true,
		},
	}

	// launch a few "go routines" "threads" to launch requests
	for i := 0; i < nWorkers; i++ {
		generator := loadGenerator{clients: clients, httpClient: httpClient}
		go func() {
			generator.GenerateLoad()
		}()
	}

	// add some stats on the console.
	go func() {
		for {
			log.Printf("Generated ticks/s: %+v\n", ticksGenerated.Swap(0))
			time.Sleep(time.Second)
		}
	}()

	time.Sleep(60 * time.Minute)
}

type loadGenerator struct {
	clients    []string
	httpClient *http.Client
}

func (l *loadGenerator) GenerateLoad() {
	for {
		ticker := l.fakeTicker()
		l.CreateTick(ticker)
		ticksGenerated.Add(1)
		time.Sleep(time.Millisecond * 5)
	}
}

func (l *loadGenerator) CreateTick(tick ticker.TickDTO) {
	buffer, err := json.Marshal(tick)
	if err != nil {
		log.Printf("error marshalling ticker %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, httpAPIURl, io.NopCloser(bytes.NewReader(buffer)))
	if err != nil {
		log.Printf("error generating request %v", err)
		return
	}

	resp, err := l.httpClient.Do(req)
	if resp != nil {
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			log.Printf("error reading body %v", err)
		}
		if err = resp.Body.Close(); err != nil {
			log.Printf("error closing body %v", err)
		}
	}

	if err != nil {
		log.Printf("error sending request %v", err)
		time.Sleep(2 * time.Second)
	}

}

// fakeTicker creates a new ticker (simulating some API)
func (l *loadGenerator) fakeTicker() ticker.TickDTO {
	chosenOne := l.clients[rand.Intn(len(l.clients))]
	futureTime := time.Duration(rand.Intn(60)) * time.Second
	return ticker.TickDTO{
		Item: chosenOne,
		At:   time.Now().Add(futureTime),
	}
}

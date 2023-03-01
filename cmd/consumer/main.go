package main

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/jlpedrosa/distributed-scheduler/internal/ticker"
)

const topicName = "ticks"
const broker = "broker:9092"
const consumerGroup = "demo-scheduler"

var ticksReceived = atomic.Int32{}

func main() {
	consumer, err := ticker.NewConsumer(broker, consumerGroup, topicName, processTick)
	if err != nil {
		panic(err)
	}

	go func() {
		consumer.Start(context.Background())
	}()

	go func() {
		for {
			log.Printf("Consumed items/s: %+v\n", ticksReceived.Swap(0))
			time.Sleep(time.Second)
		}
	}()

	time.Sleep(60 * time.Minute)

}

// processTick is the function hat should countain all the business/domain logic
func processTick(ctx context.Context, tick ticker.TickDTO) error {
	//log.Printf("Tick received! %+v\n", tick)

	ticksReceived.Add(1)
	// here we should load from the DB, by id (tick.Item)
	// and apply the logic we deem necessary
	return nil
}

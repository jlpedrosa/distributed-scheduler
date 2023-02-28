package main

import (
	"context"
	"log"
	"time"

	"github.com/jlpedrosa/distributed-scheduler/internal/ticker"
)

const topicName = "ticks"
const broker = "broker:9092"
const consumerGroup = "demo-scheduler"

func main() {
	consumer, err := ticker.NewConsumer(broker, consumerGroup, topicName, processTick)
	if err != nil {
		panic(err)
	}

	go func() {
		consumer.Start(context.Background())
	}()
	time.Sleep(60 * time.Minute)

}

func processTick(ctx context.Context, tick ticker.TickDTO) error {
	log.Printf("recived tick! %+v\n", tick)

	// here we should load from the DB, by id (tick.Item)
	// and apply the logic we deem necessary

	return nil
}
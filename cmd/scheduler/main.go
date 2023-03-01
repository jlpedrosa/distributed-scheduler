package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/jlpedrosa/distributed-scheduler/internal/ticker"
)

const setName = "scheditems"
const setCount = 10
const topicName = "ticks"
const nPartitions = int32(32)
const broker = "broker:9092"

var redisNodes = []string{"redis-node-0:6379", "redis-node-1:6379", "redis-node-2:6379"}

func main() {
	ctx := context.Background()
	ctx, cancelFn := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 10)
	signal.Notify(signalChan, os.Kill, os.Interrupt)

	go func() {
		select {
		case s := <-signalChan:
			fmt.Printf("signal %s recived", s.String())
			// Abort the context so the threads stop processing
			cancelFn()
			os.Exit(0)
		}
	}()

	// create a connection to redis, using the official driver
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    redisNodes,
		PoolSize: 20000,
	})

	// this typically shouldn't be in prod, is just to wait the dev environment
	// is actually ready.
	for redisClient.Ping(ctx).Err() != nil {
		log.Printf("Waiting for redis to be up\n")
		time.Sleep(2 * time.Second)
	}

	// this definitely should not be in prod, it just creates the topics in kafka:
	for err := errors.New(""); err != nil; err = CreateTopic(ctx, broker, topicName, nPartitions) {
		log.Printf("Unable to create topic %q\n", err)
	}

	scheduler := ticker.NewTickScheduler(redisClient, setName, setCount)
	controller := ticker.NewTickController(scheduler)

	// Start in another go routines ""thread"" the consumption from ticks from redis
	// and pushing them to kafka as their scheduled date happens
	go func() {
		redisKafkaBridge(ctx, scheduler)
	}()

	httpEngine := gin.New()
	httpEngine.Use(gin.Recovery())
	controller.ConfigureEngine(httpEngine)
	httpEngine.Run("0.0.0.0:8888")
}

func redisKafkaBridge(ctx context.Context, scheduler *ticker.TickScheduler) {
	hostname, _ := os.Hostname()

	ticksToFanOut := make(chan ticker.TickDTO, 10000)
	doneChan := ctx.Done()

	nProducers := 10
	kafkaProducers := make([]*ticker.KafkaProducer, 0, nProducers)
	for i := 0; i < nProducers; i++ {
		// create a kafka producer, using the official driver
		producer, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": broker,
			"client.id":         hostname,
			"acks":              "all",
		})
		if err != nil {
			panic(err)
		}
		kafkaProducers = append(kafkaProducers, ticker.NewKafkaProducer(producer, topicName, nPartitions))
	}

	go func() {
		scheduler.ConsumeTicks(ctx, ticksToFanOut)
		close(ticksToFanOut)
	}()

	i := 0
	for {

		select {
		case tickDTO, found := <-ticksToFanOut:
			if !found {
				//channel closed, we must return exit
				return
				fmt.Printf("no more messages, existing")
			}

			kafkaProducer := kafkaProducers[i%nProducers]
			//fmt.Printf("scheduled tick found, forwarding to kafka %+v", tickDTO)
			if err := kafkaProducer.SendTick(ctx, tickDTO); err != nil {
				fmt.Printf("error bridging to kafka: %v", err)
			}
		case <-doneChan:
			return
		}
	}

}

// CreateTopic creates a kafka topic, this clearly doesn't belong here, it just helps to simplify the PoC
func CreateTopic(ctx context.Context, broker string, topic string, numPartitions int32) error {

	log.Printf("Creating a new admin client using {%v, %v, %v,}", broker, topic, numPartitions)

	// Create a new AdminClient.
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": broker})
	if err != nil {
		return fmt.Errorf("%w unable to create kafka admin client", err)
	}

	_, err = a.CreateTopics(
		ctx,
		// Multiple topics can be created simultaneously
		// by providing more TopicSpecification structs here.
		[]kafka.TopicSpecification{{
			Topic:             topic,
			NumPartitions:     int(numPartitions),
			ReplicationFactor: 1,
		}})

	if err != nil {
		return fmt.Errorf("%w unable to create kafka topic", err)
	}
	return nil
}

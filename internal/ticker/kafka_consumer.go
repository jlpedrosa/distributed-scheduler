package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type TickConsumer struct {
	kafkaConfiguration *kafka.ConfigMap
	kafkaConsumer      *kafka.Consumer
	processor          ProcessTick

	errors   chan error
	stopChan chan struct{}
	doneChan chan struct{}
	topic    string
}

type ProcessTick func(ctx context.Context, tick TickDTO) error

// NewConsumer creates a new kafka consumer with the provided config,
func NewConsumer(broker string, consumerGroup string, topic string, processor ProcessTick) (*TickConsumer, error) {
	consumerCfg := &kafka.ConfigMap{
		"bootstrap.servers":     broker,
		"broker.address.family": "v4",
		"group.id":              consumerGroup,
		"auto.offset.reset":     "latest",
		"enable.auto.commit":    true,
	}

	eventConsumer, err := kafka.NewConsumer(consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("%w unable to create kafka consumer for safety updater", err)
	}

	return &TickConsumer{
		kafkaConfiguration: consumerCfg,
		kafkaConsumer:      eventConsumer,
		processor:          processor,
		topic:              topic,
		errors:             make(chan error),
		stopChan:           make(chan struct{}),
		doneChan:           make(chan struct{}),
	}, nil
}

// Start infinite loop that subscribes to topics and receives the messages, delegates the execution of the message
// to the provided eventProcessor
func (c *TickConsumer) Start(ctx context.Context) {
	log.Printf("Starting kafka consumer with config %+v\n", c.kafkaConfiguration)
	log.Printf("subscribing to kafka topic: %q", c.topic)

	err := c.kafkaConsumer.Subscribe(c.topic, nil)
	if err != nil {
		log.Printf("unable to subscribe to topics")
		panic(err)
	}

	shouldRun := true
	for shouldRun {
		needsReconnect := false

		for shouldRun && !needsReconnect {
			select {
			case <-c.stopChan:
				log.Println("event processor request signal received, no more messages will be processed")
				shouldRun = false
				break
			default:
				// probably this 250 is a very sensible default, but we should put it into the config
				if ev := c.kafkaConsumer.Poll(250); ev != nil {

					// TODO: find out if on case of kafka error, the driver expect, close and open,
					// this will just try to resubscribe
					if processError := c.processKafkaMessage(ctx, ev); processError != nil {
						log.Printf("error processing message %+v", processError)
						needsReconnect = true
					}
				}
			}
		}
	}

	if err := c.kafkaConsumer.Close(); err != nil {
		c.errors <- err
	}

	close(c.errors)
	c.doneChan <- struct{}{}
	close(c.doneChan)
}

func (c *TickConsumer) processKafkaMessage(ctx context.Context, ev kafka.Event) error {
	switch e := ev.(type) {
	case *kafka.Message:
		var tick TickDTO
		err := json.Unmarshal(e.Value, &tick)
		if err != nil {
			return fmt.Errorf("%w unable to deserialize kafka message", err)
		}
		c.processor(ctx, tick)
	case kafka.Error:
		return e
	default:
		log.Printf("recevived a kafka event of unknown type %v\n", e)
	}
	return nil
}
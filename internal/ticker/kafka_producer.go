package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaProducer struct {
	producer   *kafka.Producer
	topic      string
	partitions int32
}

func NewKafkaProducer(producer *kafka.Producer, topic string, partitions int32) *KafkaProducer {
	kProducer := &KafkaProducer{
		producer:   producer,
		topic:      topic,
		partitions: partitions,
	}
	go func() {
		kProducer.DumpEvents()
	}()

	return kProducer
}

func (kp *KafkaProducer) SendTick(ctx context.Context, tick TickDTO) error {
	tickBytes := tickToBytes(tick)
	partition := kp.tickToPartition(tick)

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &kp.topic, Partition: partition},
		Value:          tickBytes,
		Key:            []byte(tick.Item),
		Headers:        []kafka.Header{{Key: "content-type", Value: []byte("json")}},
	}

	//log.Printf("sending kafka message { topic:%v partition:%v }", kp.topic, partition)

	err := kp.producer.Produce(message, nil)
	if err != nil {
		return fmt.Errorf("%w unable to send tick to kafka\n", err)
	}

	return nil
}

func (kp *KafkaProducer) DumpEvents() {
	eventsChan := kp.producer.Events()

	for {
		select {
		case ev, found := <-eventsChan:
			if !found {
				fmt.Printf("event channels seems closed\n")
			}
			switch ev.(type) {
			default:
				fmt.Printf("event received: %+v", ev)
			}
		}
	}
}

func (kp *KafkaProducer) tickToPartition(tick TickDTO) int32 {
	hash := int32(0)
	b := []byte(tick.Item)
	for idx := 0; idx < len(tick.Item); idx++ {
		hash += int32(b[idx])
	}
	return hash % kp.partitions
}

func tickToBytes(tick TickDTO) []byte {
	buffer, err := json.Marshal(tick)
	if err != nil {
		panic(err)
	}
	return buffer
}

package ticker

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const maxBackOff = 8

// maxScore the maximum score representable in redis with int numbers (see: https://redis.io/commands/zadd/)
const maxScore = 9007199254740992

type TickScheduler struct {
	client  *redis.ClusterClient
	setName string
}

// NewTickScheduler creates a new redis based scheduler, using redis client to perform operations.
// the items are stored/retrieved on the set with key name provided in setName
func NewTickScheduler(client *redis.ClusterClient, setName string) *TickScheduler {
	return &TickScheduler{
		client:  client,
		setName: setName,
	}
}

// ScheduleTickAt Will persist a tick event at a given point in time if the time is before
// what is already scheduled
func (t *TickScheduler) ScheduleTickAt(ctx context.Context, key string, at time.Time) error {

	_, err := t.client.ZAdd(ctx, t.setName, redis.Z{
		Score:  timeToScore(at),
		Member: key,
	}).Result()

	if err != nil {
		return fmt.Errorf("%w unable unable to add key to set", err)
	}

	return nil
}

// ConsumeTicks Will receive the ticks, providing the key in the input channel
func (t *TickScheduler) ConsumeTicks(ctx context.Context, tickChan chan TickDTO) error {

	backoff := 0
	totalItemsRead := atomic.Int32{}

	go func() {
		for {
			log.Printf("%+v Scheduled items/s", totalItemsRead.Swap(0))
			time.Sleep(time.Second)
		}
	}()

	// this loop will only exit on errors and will be constantly consuming events
	for {
		nowAsScore := timeToScore(time.Now().UTC())
		args := redis.ZRangeBy{
			Min:    "-inf",
			Max:    fmt.Sprint(nowAsScore),
			Offset: 0,
			Count:  5,
		}

		// here we fetch only items that are expired
		items, err := t.client.ZRangeByScoreWithScores(ctx, t.setName, &args).Result()
		if err != nil {
			return fmt.Errorf("%w unable to poll items from queue", err)
		}

		toBeDeleted := make([]interface{}, 0, len(items))
		for _, item := range items {
			key, valid := (item.Member).(string)
			if !valid {
				log.Printf("unable to use key as string %v", valid)
			}
			triggerTime := scoreToTime(item.Score)

			tickChan <- TickDTO{
				Item: key,
				At:   triggerTime,
			}
			toBeDeleted = append(toBeDeleted, key)
		}

		if len(items) > 0 {
			totalItemsRead.Add(int32(len(items)))
			backoff = 0
			pipeline := t.client.Pipeline()
			pipeline.ZRem(ctx, t.setName, toBeDeleted...)
			if _, err := pipeline.Exec(ctx); err != nil {
				return fmt.Errorf("%w unable to delete feteched keys", err)
			}
		}

		if len(items) == 0 {
			backoff = backoff + 1
			if backoff > maxBackOff {
				backoff = maxBackOff
			}
			// this should be an exponential backoff
			sleepTime := time.Millisecond * 250 * time.Duration(backoff)
			time.Sleep(sleepTime)
		}
	}
	return nil
}

// timeToScore this function just transforms a time structure into epoch and float.
// redis can keep up to 52? bytes without loss of data (maxScore)
func timeToScore(t time.Time) float64 {
	tUnix := t.Unix()
	if tUnix > maxScore {
		log.Printf("warn max epoch representable reached, %v", tUnix)
	}
	return float64(tUnix)
}

func scoreToTime(score float64) time.Time {
	tUnix := int64(score)
	if tUnix > maxScore {
		log.Printf("warn max epoch representable reached, %v", tUnix)
	}
	return time.Unix(tUnix, 0)
}

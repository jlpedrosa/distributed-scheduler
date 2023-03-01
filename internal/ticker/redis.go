package ticker

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
)

const maxBackOff = 8
const maxLoops = 100

// maxScore the maximum score representable in redis with int numbers (see: https://redis.io/commands/zadd/)
const maxScore = 9007199254740992

var totalItemsRead = atomic.Int32{}

type TickScheduler struct {
	client   *redis.ClusterClient
	setName  string
	lock     *redislock.Client
	setCount int
}

// NewTickScheduler creates a new redis based scheduler, using redis client to perform operations.
// the items are stored/retrieved on the set with key name provided in setName
func NewTickScheduler(client *redis.ClusterClient, setName string, count int) *TickScheduler {
	return &TickScheduler{
		client:   client,
		setName:  setName,
		setCount: count,
		lock:     redislock.New(client),
	}
}

// ScheduleTickAt Will persist a tick event at a given point in time if the time is before
// what is already scheduled
func (t *TickScheduler) ScheduleTickAt(ctx context.Context, key string, at time.Time) error {
	_, err := t.client.ZAdd(ctx, t.getASetName(), redis.Z{
		Score:  timeToScore(at),
		Member: key,
	}).Result()

	if err != nil {
		return fmt.Errorf("%w unable unable to add key to set", err)
	}

	return nil
}

func (t *TickScheduler) ConsumeTicks(ctx context.Context, tickChan chan TickDTO) error {
	for idx := 0; idx < t.setCount; idx++ {
		setName := t.getSetName(idx)
		go func(set string) {
			for {
				if err := t.consumeSetTicks(ctx, set, tickChan); err != nil {
					log.Printf("error consuming from set %v", set)
				}
				time.Sleep(40 * time.Millisecond)
			}
		}(setName)
	}

	for {
		log.Printf("Scheduled items/s: %+v\n", totalItemsRead.Swap(0))
		time.Sleep(time.Second)
	}

	return nil
}

// ConsumeSetTicks Will receive the ticks, providing the key in the input channel
func (t *TickScheduler) consumeSetTicks(ctx context.Context, setName string, tickChan chan TickDTO) error {
	backoff := 0

	lockDuration := time.Minute
	var lock *redislock.Lock
	var err error

	for lock == nil {
		opts := &redislock.Options{
			RetryStrategy: redislock.NoRetry(),
		}
		lock, err = t.lock.Obtain(ctx, fmt.Sprintf("%s-lock", setName), lockDuration, opts)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
		}
	}
	defer lock.Release(ctx)

	// this loop will only exit on errors and will be constantly consuming events
	// or if we have locked the loop for too much, allowing other nodes on the cluster
	// to capture this particular set
	for i := 0; i < maxLoops; i++ {
		nowAsScore := timeToScore(time.Now().UTC())
		args := redis.ZRangeBy{
			Min:    "-inf",
			Max:    fmt.Sprint(nowAsScore),
			Offset: 0,
			Count:  5,
		}

		// here we fetch only items that are expired
		items, err := t.client.ZRangeByScoreWithScores(ctx, setName, &args).Result()
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
			pipeline.ZRem(ctx, setName, toBeDeleted...)
			if _, err := pipeline.Exec(ctx); err != nil {
				return fmt.Errorf("%w unable to delete feteched keys", err)
			}
		}

		if err != lock.Refresh(ctx, lockDuration, nil) {
			return fmt.Errorf("%w - unable to refresh lock", err)
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

// timeToScore  transforms a time structure into epoch and float.
// redis can keep up to 52? bytes without loss of data (maxScore)
func timeToScore(t time.Time) float64 {
	tUnix := t.Unix()
	if tUnix > maxScore {
		log.Printf("warn max epoch representable reached, %v", tUnix)
	}
	return float64(tUnix)
}

// scoreToTime transforms a unix timestamp in float mode to a time
// redis can keep up to 52? bytes without loss of data (maxScore)
func scoreToTime(score float64) time.Time {
	tUnix := int64(score)
	if tUnix > maxScore {
		log.Printf("warn max epoch representable reached, %v", tUnix)
	}
	return time.Unix(tUnix, 0)
}

func (t *TickScheduler) getASetName() string {
	return t.getSetName(rand.Intn(t.setCount))
}

func (t *TickScheduler) getSetName(index int) string {
	return fmt.Sprintf("%s-%v", t.setName, index)
}

package ticker

import "time"

type TickDTO struct {
	Item string    `json:"item"`
	At   time.Time `json:"at"`
}

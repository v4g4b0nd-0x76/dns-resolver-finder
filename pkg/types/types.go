package types

import "time"

type Resolver struct {
	IP       string        `json:"ip"`
	Latency  time.Duration `json:"latency"`
	LastTest time.Time     `json:"last_test"`
}

package stack

import (
	"time"
)

// Stack represents a PR stack
type Stack struct {
	Name       string    `json:"name"`
	Branch     string    `json:"branch"`
	Base       string    `json:"base"`
	Created    time.Time `json:"created"`
	LastSynced time.Time `json:"last_synced"`
}

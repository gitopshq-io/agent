package usecase

import (
	"time"

	"github.com/gitopshq-io/agent/internal/port"
)

func now(c port.Clock) time.Time {
	if c == nil {
		return time.Now().UTC()
	}
	return c.Now().UTC()
}

package plan

import (
	"context"
)

type Step interface {
	Execute(ctx context.Context) (Step, error)
	ID() string
}

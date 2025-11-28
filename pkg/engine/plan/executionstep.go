//go:generate mockgen -source executionstep.go -destination executionstep_mock.go -package plan
package plan

import (
	"context"
)

type Step interface {
	execute(ctx context.Context) (*stepWrapper, error)
}

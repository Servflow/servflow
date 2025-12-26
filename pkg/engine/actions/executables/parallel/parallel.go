package parallel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
)

type Config struct {
	Steps         []string `json:"steps" yaml:"steps"`
	StopOnFailure bool     `json:"stopOnFailure" yaml:"stopOnFailure"`
}

type Exec struct {
	config Config
}

func (e *Exec) Config() string {
	return ""
}

type groupError struct {
	errors               sync.Map
	count                int
	firstErrorIdentifier string
}

func (e *groupError) Error() string {
	var errorsList []string
	e.errors.Range(func(key, value any) bool {
		errorsList = append(errorsList, fmt.Sprintf("error in %s: %v", key, value))
		return true
	})
	return strings.Join(errorsList, ", ")
}

func (e *groupError) add(step string, err error) {
	e.errors.Store(step, err)
	if e.count == 0 {
		e.firstErrorIdentifier = step
	}
	e.count++
}

func (e *groupError) firstError() error {
	if e.firstErrorIdentifier == "" {
		return nil
	}
	err, ok := e.errors.Load(e.firstErrorIdentifier)
	if !ok {
		return nil
	}
	return err.(error)
}

func (e *Exec) Execute(ctx context.Context, _ string) (interface{}, error) {
	newCtx, cancel := context.WithCancel(ctx)
	type customError struct {
		err  error
		step string
	}
	// TODO think of changing the length of the buffer to n
	errChan := make(chan customError, len(e.config.Steps)-1)
	var allErrors groupError

	var wg sync.WaitGroup
	for _, step := range e.config.Steps {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			fmt.Printf("Executing step: %s\n", s)
			_, err := plan.ExecuteFromContext(newCtx, s, "")
			if err != nil {
				errChan <- customError{err: err, step: s}
			}
		}(step)
	}

	go func() {
		for err := range errChan {
			if err.err != nil && !errors.Is(err.err, plan.ErrContextCanceled) {
				allErrors.add(err.step, err.err)
				if e.config.StopOnFailure {
					cancel()
				}
			}
		}
	}()
	wg.Wait()
	close(errChan)

	if allErrors.count > 0 {
		if e.config.StopOnFailure {
			return nil, allErrors.firstError()
		} else {
			return nil, &allErrors
		}
	}
	return nil, nil
}

func (e *Exec) Type() string {
	return "parallel"
}

func init() {
	fields := map[string]actions.FieldInfo{
		"steps": {
			Type:     actions.FieldTypeArray,
			Required: true,
			Label:    "Steps to execute",
		},
		"stopOnFailure": {
			Type:    actions.FieldTypeBoolean,
			Default: true,
			Label:   "Stop On Failure",
		},
	}

	if err := actions.RegisterAction("parallel", actions.ActionRegistrationInfo{
		Name:        "Run In Parallel",
		Description: "Runs a series of specified steps and their respective next steps in parallel. If it is set to stop on failure, the step fails after the first failure",
		Fields:      fields,
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating parallel action: %v", err)
			}
			return &Exec{cfg}, nil
		},
	}); err != nil {
		panic(err)
	}
}

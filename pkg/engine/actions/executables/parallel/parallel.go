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
	errors               map[string]error
	count                int
	firstErrorIdentifier string
	mutex                sync.Mutex
}

func (e *groupError) Error() string {
	var errorsList []string
	for step, err := range e.errors {
		errorsList = append(errorsList, fmt.Sprintf("error in %s: %v", step, err))
	}
	return strings.Join(errorsList, ", ")
}

func (e *groupError) add(step string, err error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.errors == nil {
		e.errors = map[string]error{}
	}
	e.errors[step] = err
	if e.count == 0 {
		e.firstErrorIdentifier = step
	}
	e.count++
}

func (e *groupError) Count() int {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.count
}

func (e *groupError) firstError() error {
	if e.firstErrorIdentifier == "" {
		return nil
	}
	err, ok := e.errors[e.firstErrorIdentifier]
	if !ok {
		return nil
	}
	return err
}

func (e *Exec) Execute(ctx context.Context, _ string) (interface{}, error) {
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type customError struct {
		err  error
		step string
	}

	errChan := make(chan customError, len(e.config.Steps))
	var allErrors groupError

	var wg sync.WaitGroup
	for _, step := range e.config.Steps {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			fmt.Printf("Executing step: %s\n", s)
			_, err := plan.ExecuteFromContext(newCtx, s, "")
			if err != nil {
				select {
				case errChan <- customError{err: err, step: s}:
				case <-newCtx.Done():
					// Context canceled, don't block
				}
			}
		}(step)
	}

	// Process errors in a separate goroutine
	errorsDone := make(chan struct{})
	go func() {
		defer close(errorsDone)
		for err := range errChan {
			if err.err != nil && !isContextCancellationError(err.err) {
				allErrors.add(err.step, err.err)
				if e.config.StopOnFailure {
					cancel()
				}
			}
		}
	}()

	wg.Wait()
	close(errChan)

	<-errorsDone

	if allErrors.Count() > 0 {
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

func isContextCancellationError(err error) bool {
	return errors.Is(err, plan.ErrContextCanceled)
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

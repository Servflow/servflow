package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock action executable for testing
type mockActionExecutable struct {
	actionType string
	config     string
}

func (m *mockActionExecutable) Type() string {
	return m.actionType
}

func (m *mockActionExecutable) Config() string {
	return m.config
}

func (m *mockActionExecutable) Execute(ctx context.Context, modifiedConfig string) (interface{}, error) {
	return "mock response", nil
}

func TestRegisterAction(t *testing.T) {
	// Note: This test uses the registry package directly since we can't easily
	// reset the global state. In practice, we'd use dependency injection.

	t.Run("successful registration", func(t *testing.T) {
		err := RegisterAction("test-action-unique", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return &mockActionExecutable{
					actionType: "test-action-unique",
					config:     string(config),
				}, nil
			},
			Fields: map[string]FieldInfo{},
		})

		require.NoError(t, err)

		// Verify the action was registered
		types := GetRegisteredActionTypes()
		assert.Contains(t, types, "test-action-unique")
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		// First registration should succeed
		err1 := RegisterAction("duplicate-action-unique", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return &mockActionExecutable{actionType: "duplicate-action-unique"}, nil
			},
			Fields: map[string]FieldInfo{},
		})
		require.NoError(t, err1)

		// Second registration should fail
		err2 := RegisterAction("duplicate-action-unique", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return &mockActionExecutable{actionType: "duplicate-action-unique"}, nil
			},
			Fields: map[string]FieldInfo{},
		})
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "already registered")
	})
}

func TestReplaceActionType(t *testing.T) {
	// Register original action
	originalConstructor := func(config json.RawMessage) (ActionExecutable, error) {
		return &mockActionExecutable{actionType: "replaceable-action-unique", config: "original"}, nil
	}
	err := RegisterAction("replaceable-action-unique", ActionRegistrationInfo{
		Constructor: originalConstructor,
		Fields:      map[string]FieldInfo{},
	})
	require.NoError(t, err)

	// Replace with new constructor
	newConstructor := func(config json.RawMessage) (ActionExecutable, error) {
		return &mockActionExecutable{actionType: "replaceable-action-unique", config: "replaced"}, nil
	}
	ReplaceActionType("replaceable-action-unique", newConstructor)

	// Verify the replacement worked
	executable, err := GetActionExecutable("replaceable-action-unique", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "replaced", executable.Config())
}

func TestGetActionExecutable(t *testing.T) {

	t.Run("successful creation", func(t *testing.T) {
		// Register test action
		err := RegisterAction("test-get-action-unique", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return &mockActionExecutable{
					actionType: "test-get-action-unique",
					config:     string(config),
				}, nil
			},
			Fields: map[string]FieldInfo{},
		})
		require.NoError(t, err)

		// Test config
		testConfig := json.RawMessage(`{"key": "value"}`)

		// Get action executable
		executable, err := GetActionExecutable("test-get-action-unique", testConfig)
		require.NoError(t, err)
		assert.NotNil(t, executable)
		assert.Equal(t, "test-get-action-unique", executable.Type())
		assert.Equal(t, string(testConfig), executable.Config())
		assert.True(t, HasRegisteredActionType("test-get-action-unique"))
		assert.False(t, HasRegisteredActionType("test-get-action-unique-second"))
	})

	t.Run("unregistered action type", func(t *testing.T) {
		executable, err := GetActionExecutable("unknown-action-unique", json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Nil(t, executable)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("constructor error", func(t *testing.T) {
		// Register action that returns an error
		err := RegisterAction("error-action-unique", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return nil, fmt.Errorf("constructor error")
			},
			Fields: map[string]FieldInfo{},
		})
		require.NoError(t, err)

		executable, err := GetActionExecutable("error-action-unique", json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Nil(t, executable)
		assert.Contains(t, err.Error(), "constructor error")
	})
}

func TestGetRegisteredActionTypes(t *testing.T) {
	t.Run("registry has actions from previous tests", func(t *testing.T) {
		types := GetRegisteredActionTypes()
		// The registry should have actions from previous tests in this file
		assert.NotEmpty(t, types)
		// Should contain some of the actions registered in previous tests
		assert.Contains(t, types, "test-action-unique")
	})

	t.Run("can register additional actions", func(t *testing.T) {
		originalCount := len(GetRegisteredActionTypes())

		err := RegisterAction("additional-test-action", ActionRegistrationInfo{
			Constructor: func(config json.RawMessage) (ActionExecutable, error) {
				return &mockActionExecutable{actionType: "additional-test-action"}, nil
			},
			Fields: map[string]FieldInfo{},
		})
		require.NoError(t, err)

		types := GetRegisteredActionTypes()
		assert.Len(t, types, originalCount+1)
		assert.Contains(t, types, "additional-test-action")
	})
}

func TestManagerThreadSafety(t *testing.T) {
	// This test ensures the manager can handle concurrent operations
	// Register an action
	err := RegisterAction("concurrent-action-unique", ActionRegistrationInfo{
		Constructor: func(config json.RawMessage) (ActionExecutable, error) {
			return &mockActionExecutable{actionType: "concurrent-action-unique"}, nil
		},
		Fields: map[string]FieldInfo{},
	})
	require.NoError(t, err)

	// Run multiple goroutines that try to get the action executable
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := GetActionExecutable("concurrent-action-unique", json.RawMessage(`{}`))
			results <- err
		}()
	}

	// Check all results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err)
	}
}

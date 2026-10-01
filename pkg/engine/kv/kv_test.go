package kv_test

import (
	"testing"

	"github.com/Servflow/servflow/pkg/engine/kv"
	"github.com/Servflow/servflow/pkg/engine/kv/kvtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSetWithoutStore(t *testing.T) {
	kv.SetStore(nil)

	_, _, err := kv.Get("key")
	assert.ErrorIs(t, err, kv.ErrNoStore)
	assert.ErrorIs(t, kv.Set("key", "value"), kv.ErrNoStore)
}

func TestGetSetUseStore(t *testing.T) {
	kv.SetStore(kvtest.New())
	t.Cleanup(func() { kv.SetStore(nil) })

	require.NoError(t, kv.Set("key", "value"))

	got, found, err := kv.Get("key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", got)
}

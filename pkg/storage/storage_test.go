package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Initialize the client
	client, err := GetClient()
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Close the client
	client.Close()

	os.Exit(code)
}

type jsonSerializable struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

func (j *jsonSerializable) Serialize() ([]byte, error) {
	return json.Marshal(j)
}

func (j *jsonSerializable) Deserialize(bytes []byte) error {
	return json.Unmarshal(bytes, &j)
}

func TestWriteAndGetEntries(t *testing.T) {
	t.Run("write and read back entries in order", func(t *testing.T) {
		entries := []Serializable{
			&jsonSerializable{Topic: "topic1", Message: "message1"},
			&jsonSerializable{Topic: "topic1", Message: "message2"},
			&jsonSerializable{Topic: "topic2", Message: "message3"},
		}

		AppendToLog("conversations", entries...)
		require.NoError(t, SyncLog())

		gotten, err := GetLogEntriesByPrefix("conversations", 10, decodeJSONSerializable)
		require.NoError(t, err)

		expected := []any{
			&jsonSerializable{Topic: "topic1", Message: "message1"},
			&jsonSerializable{Topic: "topic1", Message: "message2"},
			&jsonSerializable{Topic: "topic2", Message: "message3"},
		}
		assert.Equal(t, expected, gotten)
	})

	t.Run("a limit smaller than the log returns its most recent entries, oldest first", func(t *testing.T) {
		entries := make([]Serializable, 0, 6)
		for i := 1; i <= 6; i++ {
			entries = append(entries, &jsonSerializable{Topic: "windowed", Message: fmt.Sprintf("message%d", i)})
		}
		AppendToLog("windowed", entries...)
		require.NoError(t, SyncLog())

		gotten, err := GetLogEntriesByPrefix("windowed", 2, decodeJSONSerializable)
		require.NoError(t, err)

		assert.Equal(t, []any{
			&jsonSerializable{Topic: "windowed", Message: "message5"},
			&jsonSerializable{Topic: "windowed", Message: "message6"},
		}, gotten)
	})

	t.Run("empty prefix returns no entries", func(t *testing.T) {
		_, err := GetLogEntriesByPrefix("", 10, decodeJSONSerializable)
		require.Error(t, err)
	})

	t.Run("a non-positive limit is refused", func(t *testing.T) {
		_, err := GetLogEntriesByPrefix("conversations", 0, decodeJSONSerializable)
		require.Error(t, err)
	})
}

func decodeJSONSerializable(data []byte) (any, error) {
	var m jsonSerializable
	err := m.Deserialize(data)
	return &m, err
}

// TestAppendToLog_KeepsEveryEntryWrittenInTheSameInstant is the regression for
// keys built from the clock alone: appends made inside one nanosecond tick
// produced the same key, and each silently replaced the last.
func TestAppendToLog_KeepsEveryEntryWrittenInTheSameInstant(t *testing.T) {
	const entries = 500

	for i := range entries {
		AppendToLog("tightloop", &jsonSerializable{Topic: "tightloop", Message: fmt.Sprintf("message%d", i)})
	}
	require.NoError(t, SyncLog())

	gotten, err := GetLogEntriesByPrefix("tightloop", entries*2, decodeJSONSerializable)
	require.NoError(t, err)
	require.Len(t, gotten, entries)

	for i, entry := range gotten {
		got := entry.(*jsonSerializable)
		assert.Equal(t, fmt.Sprintf("message%d", i), got.Message, "entry %d is out of order", i)
	}
}

// TestAppendToLog_ConcurrentProducers verifies what several requests appending
// to their own threads at once rely on: nothing is lost, and each thread reads
// back in the order that thread was written.
func TestAppendToLog_ConcurrentProducers(t *testing.T) {
	const producers, each = 8, 50

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prefix := fmt.Sprintf("producer%d", p)
			for i := range each {
				AppendToLog(prefix, &jsonSerializable{Topic: prefix, Message: fmt.Sprintf("message%d", i)})
			}
		}()
	}
	wg.Wait()
	require.NoError(t, SyncLog())

	for p := range producers {
		prefix := fmt.Sprintf("producer%d", p)
		gotten, err := GetLogEntriesByPrefix(prefix, each*2, decodeJSONSerializable)
		require.NoError(t, err)
		require.Len(t, gotten, each, "%s lost entries", prefix)

		for i, entry := range gotten {
			got := entry.(*jsonSerializable)
			assert.Equal(t, fmt.Sprintf("message%d", i), got.Message, "%s entry %d is out of order", prefix, i)
		}
	}
}

// TestSyncLog_MakesAppendsReadable is the guarantee a handler leans on when it
// hands one turn's thread over to the next: what SyncLog has returned for, a
// reader can see.
func TestSyncLog_MakesAppendsReadable(t *testing.T) {
	AppendToLog("handover", &jsonSerializable{Topic: "handover", Message: "first turn"})
	require.NoError(t, SyncLog())

	gotten, err := GetLogEntriesByPrefix("handover", 10, decodeJSONSerializable)
	require.NoError(t, err)
	require.Len(t, gotten, 1)
	assert.Equal(t, "first turn", gotten[0].(*jsonSerializable).Message)

	// A sync with nothing queued is a no-op, not a hang.
	require.NoError(t, SyncLog())
}

func TestSetAndGet(t *testing.T) {
	t.Run("set and get value", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := Set(key, value)
		require.NoError(t, err)

		retrieved, found, err := Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, retrieved)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		key := "non-existent-key"

		retrieved, found, err := Get(key)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, "", retrieved)
	})

	t.Run("set with empty key returns error", func(t *testing.T) {
		err := Set("", "some-value")
		require.Error(t, err)
	})

	t.Run("get with empty key returns error", func(t *testing.T) {
		_, _, err := Get("")
		require.Error(t, err)
	})

	t.Run("overwrite existing value", func(t *testing.T) {
		key := "overwrite-key"
		originalValue := "original-value"
		newValue := "new-value"

		// Set original value
		err := Set(key, originalValue)
		require.NoError(t, err)

		// Verify original value
		retrieved, found, err := Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, originalValue, retrieved)

		// Overwrite with new value
		err = Set(key, newValue)
		require.NoError(t, err)

		// Verify new value
		retrieved, found, err = Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, newValue, retrieved)
	})

	t.Run("set and get empty string value", func(t *testing.T) {
		key := "empty-value-key"
		value := ""

		err := Set(key, value)
		require.NoError(t, err)

		retrieved, found, err := Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, retrieved)
	})
}

type flatBufferMessage struct {
	Topic   string
	Message string
}

func (c *flatBufferMessage) Serialize() ([]byte, error) {
	b := flatbuffers.NewBuilder(256)

	nameOffset := b.CreateString(c.Topic)
	messageOffset := b.CreateString(c.Message)

	CustomMessageFormatStart(b)
	CustomMessageFormatAddName(b, nameOffset)
	CustomMessageFormatAddMessage(b, messageOffset)
	w := CustomMessageFormatEnd(b)

	b.Finish(w)
	return b.FinishedBytes(), nil

}

func (c *flatBufferMessage) Deserialize(bytes []byte) error {
	customMessage := GetRootAsCustomMessageFormat(bytes, 0)
	c.Message = string(customMessage.Message())
	c.Topic = string(customMessage.Name())
	return nil
}

func BenchmarkWriteAndGetEntriesJSON(b *testing.B) {

	expected := []*jsonSerializable{
		{Topic: "topic1", Message: "message1"},
		{Topic: "topic1", Message: "message2"},
		{Topic: "topic2", Message: "message3"},
	}
	b.Run("benchmark write to log", func(b *testing.B) {
		entries := []Serializable{
			&jsonSerializable{Topic: "topic1", Message: "message1"},
			&jsonSerializable{Topic: "topic1", Message: "message2"},
			&jsonSerializable{Topic: "topic2", Message: "message3"},
		}

		for n := 0; n < b.N; n++ {
			AppendToLog("benchmarkwritejson", entries...)
		}
		require.NoError(b, SyncLog())
	})

	var once sync.Once

	b.Run("benchmark read from log", func(b *testing.B) {
		once.Do(func() {
			entries := []Serializable{
				&jsonSerializable{Topic: "topic1", Message: "message1"},
				&jsonSerializable{Topic: "topic1", Message: "message2"},
				&jsonSerializable{Topic: "topic2", Message: "message3"},
			}
			AppendToLog("benchmarkreadjson", entries...)
			require.NoError(b, SyncLog())
		})

		for n := 0; n < b.N; n++ {
			gotten, err := GetLogEntriesByPrefix("benchmarkreadjson", 50, func(data []byte) (any, error) {
				var m jsonSerializable
				err := json.Unmarshal(data, &m)
				return &m, err
			})
			require.NoError(b, err)
			assert.Equal(b, expected, gotten)
		}
	})
}

func TestDatabaseReopening(t *testing.T) {
	t.Run("set and get work after database close", func(t *testing.T) {
		key := "reopen-test-key"
		value := "reopen-test-value"

		err := Set(key, value)
		require.NoError(t, err)

		retrieved, found, err := Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, retrieved)

		client, err := GetClient()
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		newKey := "reopen-test-key-2"
		newValue := "reopen-test-value-2"

		err = Set(newKey, newValue)
		require.NoError(t, err)

		retrieved, found, err = Get(newKey)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, newValue, retrieved)
	})

	t.Run("get works after database close", func(t *testing.T) {
		key := "get-reopen-test"
		value := "get-reopen-value"

		err := Set(key, value)
		require.NoError(t, err)

		client, err := GetClient()
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)

		retrieved, found, err := Get("non-existent-key")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, "", retrieved)
	})
}

func BenchmarkWriteAndGetEntriesFLB(b *testing.B) {

	expected := []*flatBufferMessage{
		{Topic: "topic1", Message: "message1"},
		{Topic: "topic1", Message: "message2"},
		{Topic: "topic2", Message: "message3"},
	}
	b.Run("benchmark write to log", func(b *testing.B) {
		entries := []Serializable{
			&flatBufferMessage{Topic: "topic1", Message: "message1"},
			&flatBufferMessage{Topic: "topic1", Message: "message2"},
			&flatBufferMessage{Topic: "topic2", Message: "message3"},
		}

		for n := 0; n < b.N; n++ {
			AppendToLog("benchmarkwriteflb", entries...)
		}
		require.NoError(b, SyncLog())
	})

	var once sync.Once

	b.Run("benchmark read from log", func(b *testing.B) {
		once.Do(func() {
			entries := []Serializable{
				&flatBufferMessage{Topic: "topic1", Message: "message1"},
				&flatBufferMessage{Topic: "topic1", Message: "message2"},
				&flatBufferMessage{Topic: "topic2", Message: "message3"},
			}
			AppendToLog("benchmarkreadflb", entries...)
			require.NoError(b, SyncLog())
		})

		for n := 0; n < b.N; n++ {
			gotten, err := GetLogEntriesByPrefix("benchmarkreadflb", 50, func(data []byte) (any, error) {
				fb := flatBufferMessage{}
				return &fb, fb.Deserialize(data)
			})
			require.NoError(b, err)
			assert.Equal(b, expected, gotten)
		}
	})
}

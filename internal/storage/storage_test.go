package storage

import (
	"encoding/json"
	"sync"
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getClient(t *testing.T) *Client {
	client, err := GetClient()
	require.NoError(t, err)

	return client
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
	client := getClient(t)
	defer client.Close()

	t.Run("write and read back entries in order", func(t *testing.T) {
		entries := []Serializable{
			&jsonSerializable{Topic: "topic1", Message: "message1"},
			&jsonSerializable{Topic: "topic1", Message: "message2"},
			&jsonSerializable{Topic: "topic2", Message: "message3"},
		}

		err := WriteToLog("conversations", entries)
		require.NoError(t, err)

		gotten, err := GetLogEntriesByPrefix("conversations", func(data []byte) (Serializable, error) {
			var m jsonSerializable
			err := m.Deserialize(data)
			return &m, err
		})
		require.NoError(t, err)

		expected := []Serializable{
			&jsonSerializable{Topic: "topic1", Message: "message1"},
			&jsonSerializable{Topic: "topic1", Message: "message2"},
			&jsonSerializable{Topic: "topic2", Message: "message3"},
		}
		assert.Equal(t, expected, gotten)
	})

	t.Run("empty prefix returns no entries", func(t *testing.T) {
		_, err := GetLogEntriesByPrefix("", func(data []byte) (Serializable, error) {
			var m jsonSerializable
			err := m.Deserialize(data)
			return &m, err
		})
		require.Error(t, err)
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
			err := WriteToLog("benchmarkwritejson", entries)
			require.NoError(b, err)
		}
	})

	var once sync.Once

	b.Run("benchmark read from log", func(b *testing.B) {
		once.Do(func() {
			entries := []Serializable{
				&jsonSerializable{Topic: "topic1", Message: "message1"},
				&jsonSerializable{Topic: "topic1", Message: "message2"},
				&jsonSerializable{Topic: "topic2", Message: "message3"},
			}
			err := WriteToLog("benchmarkreadjson", entries)
			require.NoError(b, err)
		})

		for n := 0; n < b.N; n++ {
			gotten, err := GetLogEntriesByPrefix("benchmarkreadjson", func(data []byte) (Serializable, error) {
				var m jsonSerializable
				err := json.Unmarshal(data, &m)
				return &m, err
			})
			require.NoError(b, err)
			assert.Equal(b, expected, gotten)
		}
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
			err := WriteToLog("benchmarkwriteflb", entries)
			require.NoError(b, err)
		}
	})

	var once sync.Once

	b.Run("benchmark read from log", func(b *testing.B) {
		once.Do(func() {
			entries := []Serializable{
				&flatBufferMessage{Topic: "topic1", Message: "message1"},
				&flatBufferMessage{Topic: "topic1", Message: "message2"},
				&flatBufferMessage{Topic: "topic2", Message: "message3"},
			}
			err := WriteToLog("benchmarkreadflb", entries)
			require.NoError(b, err)
		})

		for n := 0; n < b.N; n++ {
			gotten, err := GetLogEntriesByPrefix("benchmarkreadflb", func(data []byte) (Serializable, error) {
				fb := flatBufferMessage{}
				return &fb, fb.Deserialize(data)
			})
			require.NoError(b, err)
			assert.Equal(b, expected, gotten)
		}
	})
}

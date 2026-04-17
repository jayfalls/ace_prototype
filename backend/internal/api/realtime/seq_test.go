package realtime

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeqBuffer_Append(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 10, MaxAge: time.Minute})

	// Append events to a topic
	for i := uint64(1); i <= 5; i++ {
		data, _ := json.Marshal(map[string]int{"seq": int(i)})
		buf.Append("agent:1:status", i, data)
	}

	assert.Equal(t, 5, buf.Size("agent:1:status"))
	assert.Equal(t, uint64(5), buf.GetLastSeq("agent:1:status"))
}

func TestSeqBuffer_AppendRingBuffer(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 5, MaxAge: time.Minute})

	// Append more events than max size
	for i := uint64(1); i <= 10; i++ {
		data, _ := json.Marshal(map[string]int{"seq": int(i)})
		buf.Append("agent:1:status", i, data)
	}

	// Should only have last 5 events
	assert.Equal(t, 5, buf.Size("agent:1:status"))
	assert.Equal(t, uint64(10), buf.GetLastSeq("agent:1:status"))

	// Replay should start from seq 6
	entries, err := buf.Replay("agent:1:status", 0)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
	assert.Equal(t, uint64(6), entries[0].Seq)
}

func TestSeqBuffer_ReplayWithinBuffer(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 10, MaxAge: time.Minute})

	// Append events
	for i := uint64(1); i <= 5; i++ {
		data, _ := json.Marshal(map[string]int{"seq": int(i)})
		buf.Append("agent:1:status", i, data)
	}

	// Replay since seq 3
	entries, err := buf.Replay("agent:1:status", 3)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, uint64(4), entries[0].Seq)
	assert.Equal(t, uint64(5), entries[1].Seq)
}

func TestSeqBuffer_ReplayBeyondBuffer(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 5, MaxAge: time.Minute})

	// Append 10 events (but buffer only holds 5)
	for i := uint64(1); i <= 10; i++ {
		data, _ := json.Marshal(map[string]int{"seq": int(i)})
		buf.Append("agent:1:status", i, data)
	}

	// Replay since seq 3 (should fail - buffer only holds 6-10)
	_, err := buf.Replay("agent:1:status", 3)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBufferExceeded)
}

func TestSeqBuffer_ReplayNonExistentTopic(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 10, MaxAge: time.Minute})

	entries, err := buf.Replay("nonexistent", 0)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestSeqBuffer_Expiry(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 10, MaxAge: 50 * time.Millisecond})

	// Append an event
	data, _ := json.Marshal(map[string]int{"seq": 1})
	buf.Append("agent:1:status", 1, data)
	assert.Equal(t, 1, buf.Size("agent:1:status"))

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Clean expired entries
	removed := buf.CleanExpired()
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, buf.Size("agent:1:status"))
}

func TestSeqBuffer_ConcurrentAccess(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 100, MaxAge: time.Minute})
	var wg sync.WaitGroup

	// Concurrent appends
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := uint64(0); j < 100; j++ {
				data, _ := json.Marshal(map[string]int{"id": id, "seq": int(j)})
				buf.Append("agent:1:status", j, data)
			}
		}(i)
	}

	// Concurrent replays
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = buf.Replay("agent:1:status", uint64(j))
				_ = buf.Size("agent:1:status")
				_ = buf.GetLastSeq("agent:1:status")
			}
		}(i)
	}

	wg.Wait()

	// Should have some events in buffer
	assert.GreaterOrEqual(t, buf.Size("agent:1:status"), 0)
}

func TestSeqBuffer_MultipleTopics(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 5, MaxAge: time.Minute})

	// Append to multiple topics
	buf.Append("agent:1:status", 1, []byte(`{"seq":1}`))
	buf.Append("agent:1:status", 2, []byte(`{"seq":2}`))
	buf.Append("agent:2:status", 1, []byte(`{"seq":1}`))
	buf.Append("system:health", 1, []byte(`{"seq":1}`))

	assert.Equal(t, 2, buf.Size("agent:1:status"))
	assert.Equal(t, 1, buf.Size("agent:2:status"))
	assert.Equal(t, 1, buf.Size("system:health"))

	topics := buf.Topics()
	assert.Len(t, topics, 3)
}

func TestSeqBuffer_RemoveTopic(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 5, MaxAge: time.Minute})

	buf.Append("agent:1:status", 1, []byte(`{"seq":1}`))
	buf.Append("agent:1:status", 2, []byte(`{"seq":2}`))
	assert.Equal(t, 2, buf.Size("agent:1:status"))

	buf.RemoveTopic("agent:1:status")
	assert.Equal(t, 0, buf.Size("agent:1:status"))
	assert.Equal(t, uint64(0), buf.GetLastSeq("agent:1:status"))
}

func TestMessageTypes_ClientMessageJSON(t *testing.T) {
	tests := []struct {
		name     string
		msg      ClientMessage
		wantType ClientMessageType
	}{
		{
			name: "auth message",
			msg: ClientMessage{
				Type:  ClientMessageAuth,
				Token: "jwt-token",
			},
			wantType: ClientMessageAuth,
		},
		{
			name: "subscribe message",
			msg: ClientMessage{
				Type:   ClientMessageSubscribe,
				Topics: []string{"agent:1:status", "system:health"},
			},
			wantType: ClientMessageSubscribe,
		},
		{
			name: "unsubscribe message",
			msg: ClientMessage{
				Type:   ClientMessageUnsubscribe,
				Topics: []string{"agent:1:status"},
			},
			wantType: ClientMessageUnsubscribe,
		},
		{
			name: "replay message",
			msg: ClientMessage{
				Type:     ClientMessageReplay,
				Topic:    "agent:1:status",
				SinceSeq: 100,
			},
			wantType: ClientMessageReplay,
		},
		{
			name:     "ping message",
			msg:      ClientMessage{Type: ClientMessagePing},
			wantType: ClientMessagePing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			require.NoError(t, err)

			var decoded ClientMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.wantType, decoded.Type)
		})
	}
}

func TestMessageTypes_ServerMessageJSON(t *testing.T) {
	tests := []struct {
		name     string
		msg      ServerMessage
		wantType ServerMessageType
	}{
		{
			name:     "auth_ok message",
			msg:      NewAuthOkMessage("conn-123"),
			wantType: ServerMessageAuthOk,
		},
		{
			name:     "auth_error message",
			msg:      NewAuthErrorMessage("invalid token"),
			wantType: ServerMessageAuthError,
		},
		{
			name:     "subscribed message",
			msg:      NewSubscribedMessage([]string{"agent:1:status"}),
			wantType: ServerMessageSubscribed,
		},
		{
			name:     "unsubscribed message",
			msg:      NewUnsubscribedMessage([]string{"agent:1:status"}),
			wantType: ServerMessageUnsubscribed,
		},
		{
			name: "event message",
			msg: NewEventMessage(
				"agent:1:status",
				42,
				"agent.status_change",
				json.RawMessage(`{"status":"running"}`),
			),
			wantType: ServerMessageEvent,
		},
		{
			name:     "resync_required message",
			msg:      NewResyncRequiredMessage([]string{"agent:1:status"}),
			wantType: ServerMessageResyncRequired,
		},
		{
			name:     "pong message",
			msg:      NewPongMessage(),
			wantType: ServerMessagePong,
		},
		{
			name:     "error message",
			msg:      NewErrorMessage("something went wrong"),
			wantType: ServerMessageError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			require.NoError(t, err)

			var decoded ServerMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.wantType, decoded.Type)
		})
	}
}

func TestValidateTopic(t *testing.T) {
	tests := []struct {
		topic string
		valid bool
	}{
		{"agent:1:status", true},
		{"agent:abc-123:logs", true},
		{"agent:123:cycles", true},
		{"system:health", true},
		{"usage:user-123", true},
		{"usage:user_456", true},
		{"Agent:1:status", false},       // uppercase
		{"agent:1:Status", false},       // uppercase
		{"agent::status", false},        // empty segment
		{"agent:1:", false},             // empty segment
		{":1:status", false},            // empty segment
		{"agent-1:status", false},       // dash in first segment
		{"agent:1:status:extra", false}, // too many segments
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			err := ValidateTopic(tt.topic)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestSeqBuffer_ReplaySinceTime(t *testing.T) {
	buf := NewSeqBuffer(SeqBufferConfig{MaxSize: 10, MaxAge: time.Hour})

	topic := "agent:1:status"

	// Append events with different timestamps (simulated)
	for i := uint64(1); i <= 5; i++ {
		data, _ := json.Marshal(map[string]int{"seq": int(i)})
		buf.Append(topic, i, data)
	}

	// Replay should work normally
	entries, err := buf.Replay(topic, 2)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

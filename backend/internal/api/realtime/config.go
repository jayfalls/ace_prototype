package realtime

import "time"

// WebSocket configuration constants.
const (
	// WS_AUTH_TIMEOUT is the time allowed for auth handshake.
	WS_AUTH_TIMEOUT = 5 * time.Second

	// WS_MAX_SUBSCRIPTIONS is the max topics per connection.
	WS_MAX_SUBSCRIPTIONS = 50

	// WS_MAX_MESSAGE_SIZE is the max message size in bytes.
	WS_MAX_MESSAGE_SIZE = 64 * 1024

	// WS_HEARTBEAT_INTERVAL is the interval for heartbeat checks.
	WS_HEARTBEAT_INTERVAL = 30 * time.Second

	// WS_SEND_CHANNEL_SIZE is the buffered channel size for outgoing messages.
	WS_SEND_CHANNEL_SIZE = 128

	// WS_RATE_LIMIT is the max messages per second per connection.
	WS_RATE_LIMIT = 100

	// BUFFER_MAX_SIZE is the max entries per topic in the sequence buffer.
	BUFFER_MAX_SIZE = 1000

	// BUFFER_MAX_AGE is the max age of entries in the sequence buffer.
	BUFFER_MAX_AGE = 10 * time.Minute

	// POLL_MAX_TOPICS is the max topics per polling request.
	POLL_MAX_TOPICS = 50

	// POLL_RATE_LIMIT is the max polling requests per minute per user.
	POLL_RATE_LIMIT = 60
)

// Realtime metric names.
const (
	MetricWSConnectionsActive  = "ace.realtime.ws.connections.active"
	MetricWSMessagesSent       = "ace.realtime.ws.messages.sent"
	MetricWSMessagesReceived   = "ace.realtime.ws.messages.received"
	MetricWSErrors             = "ace.realtime.ws.errors"
	MetricPollRequests         = "ace.realtime.poll.requests"
	MetricPollEventsDelivered  = "ace.realtime.poll.events.delivered"
	MetricBufferReplayEvents   = "ace.realtime.buffer.replay.events"
	MetricBufferResyncRequired = "ace.realtime.buffer.resync.required"
)

// OTel span names.
const (
	SpanWSUpgrade    = "realtime.ws.upgrade"
	SpanWSAuth       = "realtime.ws.auth"
	SpanWSSubscribe  = "realtime.ws.subscribe"
	SpanWSDisconnect = "realtime.ws.disconnect"
	SpanPoll         = "realtime.poll"
)

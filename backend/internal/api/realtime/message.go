package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Topic format: agent:{id}:status, agent:{id}:logs, agent:{id}:cycles, system:health, usage:{id}
// Format: resourceType(:resourceId(:subType)?)?
var (
	TopicRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(:[a-z0-9_-]+(:[a-z_][a-z0-9_]*)?)?$`)
)

// ValidateTopic checks if a topic string matches the expected format.
func ValidateTopic(topic string) error {
	if !TopicRegex.MatchString(topic) {
		return fmt.Errorf("invalid topic format: %q", topic)
	}
	return nil
}

// ClientMessageType represents the type of a client message.
type ClientMessageType string

const (
	ClientMessageAuth        ClientMessageType = "auth"
	ClientMessageSubscribe   ClientMessageType = "subscribe"
	ClientMessageUnsubscribe ClientMessageType = "unsubscribe"
	ClientMessageReplay      ClientMessageType = "replay"
	ClientMessagePing        ClientMessageType = "ping"
)

// ServerMessageType represents the type of a server message.
type ServerMessageType string

const (
	ServerMessageAuthOk         ServerMessageType = "auth_ok"
	ServerMessageAuthError      ServerMessageType = "auth_error"
	ServerMessageSubscribed     ServerMessageType = "subscribed"
	ServerMessageUnsubscribed   ServerMessageType = "unsubscribed"
	ServerMessageEvent          ServerMessageType = "event"
	ServerMessageResyncRequired ServerMessageType = "resync_required"
	ServerMessagePong           ServerMessageType = "pong"
	ServerMessageError          ServerMessageType = "error"
)

// BaseMessage contains common fields for all messages.
type BaseMessage struct {
	Type  string `json:"type"`
	Topic string `json:"topic,omitempty"`
	Seq   uint64 `json:"seq,omitempty"`
}

// ClientMessage represents a message sent from the client to the server.
type ClientMessage struct {
	Type      ClientMessageType `json:"type"`
	Topic     string            `json:"topic,omitempty"`
	Topics    []string          `json:"topics,omitempty"`
	Seq       uint64            `json:"seq,omitempty"`
	Data      json.RawMessage   `json:"data,omitempty"`
	Token     string            `json:"token,omitempty"`
	SinceSeq  uint64            `json:"since_seq,omitempty"`
	SinceTime int64             `json:"since_time,omitempty"`
}

// AuthClientMessage is the auth message payload.
type AuthClientMessage struct {
	Token string `json:"token"`
}

// MarshalJSON implements json.Marshaler for ClientMessage.
func (c ClientMessage) MarshalJSON() ([]byte, error) {
	if c.Type == "" {
		c.Type = ClientMessagePing
	}
	type alias ClientMessage
	return json.Marshal(alias(c))
}

// ServerMessage represents a message sent from the server to the client.
type ServerMessage struct {
	Type         ServerMessageType `json:"type"`
	Topic        string            `json:"topic,omitempty"`
	Topics       []string          `json:"topics,omitempty"`
	Seq          uint64            `json:"seq,omitempty"`
	Data         json.RawMessage   `json:"data,omitempty"`
	Error        string            `json:"error,omitempty"`
	ConnectionID string            `json:"connection_id,omitempty"`
	Resync       []string          `json:"resync_required,omitempty"`
}

// EventData represents the data payload of an event message.
type EventData struct {
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
}

// Marshal implements json.Marshaler for EventData.
func (e EventData) Marshal() json.RawMessage {
	data, _ := json.Marshal(e)
	return data
}

// NewAuthOkMessage creates an auth_ok server message.
func NewAuthOkMessage(connectionID string) ServerMessage {
	return ServerMessage{
		Type:         ServerMessageAuthOk,
		ConnectionID: connectionID,
	}
}

// NewAuthErrorMessage creates an auth_error server message.
func NewAuthErrorMessage(err string) ServerMessage {
	return ServerMessage{
		Type:  ServerMessageAuthError,
		Error: err,
	}
}

// NewSubscribedMessage creates a subscribed server message.
func NewSubscribedMessage(topics []string) ServerMessage {
	return ServerMessage{
		Type:   ServerMessageSubscribed,
		Topics: topics,
	}
}

// NewUnsubscribedMessage creates an unsubscribed server message.
func NewUnsubscribedMessage(topics []string) ServerMessage {
	return ServerMessage{
		Type:   ServerMessageUnsubscribed,
		Topics: topics,
	}
}

// NewEventMessage creates an event server message.
func NewEventMessage(topic string, seq uint64, eventType string, data json.RawMessage) ServerMessage {
	return ServerMessage{
		Type:  ServerMessageEvent,
		Topic: topic,
		Seq:   seq,
		Data: EventData{
			EventType: eventType,
			Data:      data,
		}.Marshal(),
	}
}

// NewResyncRequiredMessage creates a resync_required server message.
func NewResyncRequiredMessage(topics []string) ServerMessage {
	return ServerMessage{
		Type:   ServerMessageResyncRequired,
		Resync: topics,
	}
}

// NewPongMessage creates a pong server message.
func NewPongMessage() ServerMessage {
	return ServerMessage{
		Type: ServerMessagePong,
	}
}

// NewErrorMessage creates an error server message.
func NewErrorMessage(err string) ServerMessage {
	return ServerMessage{
		Type:  ServerMessageError,
		Error: err,
	}
}

// Marshal implements json.Marshaler for ServerMessage.
func (s ServerMessage) Marshal() json.RawMessage {
	data, _ := json.Marshal(s)
	return data
}

// ErrInvalidTopic is returned when a topic format is invalid.
var ErrInvalidTopic = errors.New("invalid topic format")

// ErrBufferExceeded is returned when the sequence buffer cannot hold more events.
var ErrBufferExceeded = errors.New("buffer exceeded")

package messaging

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	mockClient := &MockClient{}

	err := Publish(mockClient, "test.subject", "corr-id-123", "agent-1", "cycle-1", "test-service", []byte(`{"test": true}`))

	require.NoError(t, err)
	require.Len(t, mockClient.PublishedMsgs, 1)

	msg := mockClient.PublishedMsgs[0]
	assert.Equal(t, "test.subject", msg.Subject)
	assert.Equal(t, `{"test": true}`, string(msg.Data))
	assert.NotEmpty(t, msg.Headers.Get(HeaderMessageID))
	assert.Equal(t, "corr-id-123", msg.Headers.Get(HeaderCorrelationID))
	assert.Equal(t, "agent-1", msg.Headers.Get(HeaderAgentID))
	assert.Equal(t, "cycle-1", msg.Headers.Get(HeaderCycleID))
	assert.Equal(t, "test-service", msg.Headers.Get(HeaderSourceService))
	assert.NotEmpty(t, msg.Headers.Get(HeaderTimestamp))
	assert.Equal(t, SchemaVersion, msg.Headers.Get(HeaderSchemaVersion))
}

func TestPublishWithSubject(t *testing.T) {
	mockClient := &MockClient{}

	err := PublishWithSubject(mockClient, SubjectToolsInvoke, "corr-id", "agent-1", "cycle-1", "test-service", []byte(`{"tool": "browse"}`), "agent-1", "browse")

	require.NoError(t, err)
	require.Len(t, mockClient.PublishedMsgs, 1)

	msg := mockClient.PublishedMsgs[0]
	assert.Equal(t, "ace.tools.agent-1.browse.invoke", msg.Subject)
}

func TestRequestReply(t *testing.T) {
	mockClient := &MockClient{
		RequestResp: &nats.Msg{
			Subject: "response.subject",
			Data:    []byte(`{"result": "success"}`),
			Header:  nats.Header{},
		},
	}

	mockClient.RequestResp.Header.Set(HeaderCorrelationID, "corr-id-456")

	result, err := RequestReply(mockClient, "test.subject", "corr-id-123", "agent-1", "cycle-1", "test-service", []byte(`{"request": true}`), 5*time.Second)

	require.NoError(t, err)
	assert.Equal(t, `{"result": "success"}`, string(result))

	// Verify the request was published
	require.Len(t, mockClient.PublishedMsgs, 0) // Request uses client.Request, not Publish

	// Check that we have a subscription (RequestReply calls client.Request)
	// The MockClient's Request method is tested here
}

func TestRequestReplyWithSubject(t *testing.T) {
	mockClient := &MockClient{
		RequestResp: &nats.Msg{
			Subject: "response.subject",
			Data:    []byte(`{"result": "success"}`),
		},
	}

	result, err := RequestReplyWithSubject(mockClient, SubjectToolsInvoke, "corr-id", "agent-1", "cycle-1", "test-service", []byte(`{"tool": "browse"}`), 5*time.Second, "agent-1", "browse")

	require.NoError(t, err)
	assert.Equal(t, `{"result": "success"}`, string(result))
}

func TestRequestReplyDefault(t *testing.T) {
	mockClient := &MockClient{
		RequestResp: &nats.Msg{
			Subject: "response.subject",
			Data:    []byte(`{"result": "ok"}`),
		},
	}

	result, err := RequestReplyDefault(mockClient, "test.subject", "corr-id", "agent-1", "cycle-1", "test-service", []byte(`{"test": true}`))

	require.NoError(t, err)
	assert.Equal(t, `{"result": "ok"}`, string(result))
}

func TestRequestReplyTimeout(t *testing.T) {
	mockClient := &MockClient{
		RequestErr: &MessagingError{Code: ErrCodeTimeout, Message: "operation timed out"},
	}

	_, err := RequestReply(mockClient, "test.subject", "corr-id", "agent-1", "cycle-1", "test-service", []byte(`{"test": true}`), 5*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "TIMEOUT")
}

func TestSetHeadersToMsg(t *testing.T) {
	env := NewEnvelope("corr-id", "agent-1", "cycle-1", "test-service")
	env.SetPayload(map[string]string{"key": "value"})

	headers := make(nats.Header)
	SetHeadersToMsg(headers, env)

	assert.Equal(t, env.MessageID, headers.Get(HeaderMessageID))
	assert.Equal(t, "corr-id", headers.Get(HeaderCorrelationID))
	assert.Equal(t, "agent-1", headers.Get(HeaderAgentID))
	assert.Equal(t, "cycle-1", headers.Get(HeaderCycleID))
	assert.Equal(t, "test-service", headers.Get(HeaderSourceService))
	assert.NotEmpty(t, headers.Get(HeaderTimestamp))
	assert.Equal(t, SchemaVersion, headers.Get(HeaderSchemaVersion))
}

func TestSetHeadersToMsgNil(t *testing.T) {
	// Test with nil inputs - should not panic
	env := NewEnvelope("corr-id", "agent-1", "cycle-1", "test-service")

	assert.NotPanics(t, func() {
		SetHeadersToMsg(nil, env)
	})

	assert.NotPanics(t, func() {
		SetHeadersToMsg(make(nats.Header), nil)
	})

	assert.NotPanics(t, func() {
		SetHeadersToMsg(nil, nil)
	})
}

func TestSubscribe(t *testing.T) {
	mockClient := &MockClient{}

	handlerCalled := false
	handler := func(msg *nats.Msg) error {
		handlerCalled = true
		return nil
	}

	sub, err := Subscribe(mockClient, "test.subject", handler)

	require.NoError(t, err)
	require.NotNil(t, sub)

	// Simulate receiving a message
	require.Len(t, mockClient.Subscriptions, 1)
	subObj := mockClient.Subscriptions[0]
	assert.Equal(t, "test.subject", subObj.Subject)

	// Call the handler
	subObj.Handler(&nats.Msg{Subject: "test.subject", Data: []byte(`{"test": true}`)})
	assert.True(t, handlerCalled)
}

func TestSubscribeWithEnvelope(t *testing.T) {
	mockClient := &MockClient{}

	envReceived := false
	var receivedEnv *Envelope
	handler := func(env *Envelope, data []byte) error {
		envReceived = true
		receivedEnv = env
		return nil
	}

	sub, err := SubscribeWithEnvelope(mockClient, "test.subject", handler)

	require.NoError(t, err)
	require.NotNil(t, sub)

	// Simulate receiving a message with headers
	require.Len(t, mockClient.Subscriptions, 1)
	subObj := mockClient.Subscriptions[0]

	msg := &nats.Msg{
		Subject: "test.subject",
		Data:    []byte(`{"test": true}`),
		Header: nats.Header{
			HeaderMessageID:     []string{"msg-id-123"},
			HeaderCorrelationID: []string{"corr-id-456"},
			HeaderAgentID:       []string{"agent-1"},
			HeaderCycleID:       []string{"cycle-1"},
			HeaderSourceService: []string{"test-service"},
		},
	}

	subObj.Handler(msg)
	assert.True(t, envReceived)
	assert.NotNil(t, receivedEnv)
	assert.Equal(t, "msg-id-123", receivedEnv.MessageID)
	assert.Equal(t, "corr-id-456", receivedEnv.CorrelationID)
	assert.Equal(t, "agent-1", receivedEnv.AgentID)
}

func TestCreateRequestEnvelope(t *testing.T) {
	// Test with incoming message that has correlation ID
	incomingMsg := &nats.Msg{
		Subject: "test.subject",
		Header: nats.Header{
			HeaderCorrelationID: []string{"existing-corr-id"},
			HeaderSourceService: []string{"test-service"},
		},
	}

	env := CreateRequestEnvelope(incomingMsg, "agent-1", "cycle-1", "response-service")

	assert.Equal(t, "existing-corr-id", env.CorrelationID)
	assert.Equal(t, "agent-1", env.AgentID)
	assert.Equal(t, "cycle-1", env.CycleID)
	assert.Equal(t, "response-service", env.SourceService)
	assert.NotEmpty(t, env.MessageID)
}

func TestCreateRequestEnvelopeNoIncoming(t *testing.T) {
	// Test with nil incoming message - should generate new correlation ID
	env := CreateRequestEnvelope(nil, "agent-1", "cycle-1", "response-service")

	assert.NotEmpty(t, env.CorrelationID)
	assert.Equal(t, "agent-1", env.AgentID)
	assert.Equal(t, "cycle-1", env.CycleID)
	assert.Equal(t, "response-service", env.SourceService)
}

func TestCreateRequestEnvelopeNoCorrelationID(t *testing.T) {
	// Test with incoming message without correlation ID
	incomingMsg := &nats.Msg{
		Subject: "test.subject",
		Header: nats.Header{
			HeaderSourceService: []string{"test-service"},
		},
	}

	env := CreateRequestEnvelope(incomingMsg, "agent-1", "cycle-1", "response-service")

	// Should generate new correlation ID since none exists
	assert.NotEmpty(t, env.CorrelationID)
}

func TestReplyTo(t *testing.T) {
	mockClient := &MockClient{}

	incomingMsg := &nats.Msg{
		Subject: "test.subject",
		Reply:   "test.reply.subject",
		Header: nats.Header{
			HeaderCorrelationID: []string{"corr-id-123"},
			HeaderSourceService: []string{"test-service"},
		},
	}

	err := ReplyTo(mockClient, incomingMsg, []byte(`{"response": true}`))

	require.NoError(t, err)
	require.Len(t, mockClient.PublishedMsgs, 1)

	msg := mockClient.PublishedMsgs[0]
	assert.Equal(t, "test.reply.subject", msg.Subject)
	assert.Equal(t, "corr-id-123", msg.Headers.Get(HeaderCorrelationID))
}

func TestReplyToNilMessage(t *testing.T) {
	mockClient := &MockClient{}

	err := ReplyTo(mockClient, nil, []byte(`{"response": true}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestReplyToNoReplySubject(t *testing.T) {
	mockClient := &MockClient{}

	incomingMsg := &nats.Msg{
		Subject: "test.subject",
		Reply:   "", // No reply subject
		Header: nats.Header{
			HeaderCorrelationID: []string{"corr-id-123"},
		},
	}

	err := ReplyTo(mockClient, incomingMsg, []byte(`{"response": true}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reply subject")
}

func TestForwardMessage(t *testing.T) {
	mockClient := &MockClient{}

	incomingMsg := &nats.Msg{
		Subject: "original.subject",
		Data:    []byte(`{"forwarded": true}`),
		Header: nats.Header{
			HeaderMessageID:     []string{"msg-id-123"},
			HeaderCorrelationID: []string{"corr-id-456"},
			HeaderAgentID:       []string{"agent-1"},
			HeaderSourceService: []string{"test-service"},
		},
	}

	err := ForwardMessage(mockClient, incomingMsg, "new.subject")

	require.NoError(t, err)
	require.Len(t, mockClient.PublishedMsgs, 1)

	msg := mockClient.PublishedMsgs[0]
	assert.Equal(t, "new.subject", msg.Subject)
	assert.Equal(t, `{"forwarded": true}`, string(msg.Data))
	assert.Equal(t, "corr-id-456", msg.Headers.Get(HeaderCorrelationID))
	assert.Equal(t, "agent-1", msg.Headers.Get(HeaderAgentID))
}

func TestForwardMessageNilMessage(t *testing.T) {
	mockClient := &MockClient{}

	err := ForwardMessage(mockClient, nil, "new.subject")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestSubscribeToStream(t *testing.T) {
	mockClient := &MockClient{}

	handlerCalled := false
	handler := func(msg *nats.Msg) error {
		handlerCalled = true
		return nil
	}

	err := SubscribeToStream(nil, mockClient, "TEST_STREAM", "test-consumer", "test.subject", handler)

	require.NoError(t, err)
	require.Len(t, mockClient.StreamSubs, 1)

	subObj := mockClient.StreamSubs[0]
	assert.Equal(t, "TEST_STREAM", subObj.Stream)
	assert.Equal(t, "test-consumer", subObj.Consumer)
	assert.Equal(t, "test.subject", subObj.Subject)

	// Test the handler
	subObj.Handler(&nats.Msg{Subject: "test.subject", Data: []byte(`{"test": true}`)})
	assert.True(t, handlerCalled)
}

func TestSubscribeToStreamWithSubject(t *testing.T) {
	mockClient := &MockClient{}

	err := SubscribeToStreamWithSubject(nil, mockClient, "TEST_STREAM", "test-consumer", SubjectEngineLayerInput, func(msg *nats.Msg) error {
		return nil
	}, "agent-1", "layer-1")

	require.NoError(t, err)
	require.Len(t, mockClient.StreamSubs, 1)

	subObj := mockClient.StreamSubs[0]
	assert.Equal(t, "ace.engine.agent-1.layer.layer-1.input", subObj.Subject)
}

func TestStreamSubscriptionConfig(t *testing.T) {
	mockClient := &MockClient{}

	cfg := StreamSubscriptionConfig{
		Stream:     "TEST_STREAM",
		Consumer:   "test-consumer",
		Subject:    "test.subject",
		AutoAck:    true,
		MaxDeliver: 5,
	}

	handlerCalled := false
	cfg.Handler = func(msg *nats.Msg) error {
		handlerCalled = true
		return nil
	}

	err := SubscribeToStreamWithConfig(nil, mockClient, cfg)

	require.NoError(t, err)
	require.Len(t, mockClient.StreamSubs, 1)

	// Simulate a message arriving - call the stored handler
	subObj := mockClient.StreamSubs[0]
	subObj.Handler(&nats.Msg{Subject: "test.subject", Data: []byte(`{"test": true}`)})
	assert.True(t, handlerCalled)
}

func TestPublishPreservesCorrelationID(t *testing.T) {
	mockClient := &MockClient{}

	// First publish
	err := Publish(mockClient, "subject1", "corr-id-123", "agent-1", "cycle-1", "service-a", []byte(`{"step": 1}`))
	require.NoError(t, err)

	msg1 := mockClient.PublishedMsgs[0]
	assert.Equal(t, "corr-id-123", msg1.Headers.Get(HeaderCorrelationID))

	// Second publish should preserve same correlation ID
	err = Publish(mockClient, "subject2", "corr-id-123", "agent-1", "cycle-1", "service-b", []byte(`{"step": 2}`))
	require.NoError(t, err)

	msg2 := mockClient.PublishedMsgs[1]
	assert.Equal(t, "corr-id-123", msg2.Headers.Get(HeaderCorrelationID))
}

func TestVariadicSubjectFormatting(t *testing.T) {
	// Test variadic subject formatting
	subject1 := SubjectToolsInvoke.Format("agent-1", "tool-1")
	assert.Equal(t, "ace.tools.agent-1.tool-1.invoke", subject1)

	subject2 := SubjectEngineLayerInput.Format("agent-1", "layer-1")
	assert.Equal(t, "ace.engine.agent-1.layer.layer-1.input", subject2)

	subject3 := SubjectSystemHealth.Format("service-name")
	assert.Equal(t, "ace.system.health.service-name", subject3)
}

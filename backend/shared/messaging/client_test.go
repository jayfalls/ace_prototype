package messaging

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		URLs:          "nats://localhost:4222",
		Name:          "test-client",
		Timeout:       10 * time.Second,
		MaxReconnect:  10,
		ReconnectWait: 1 * time.Second,
	}

	assert.Equal(t, "nats://localhost:4222", cfg.URLs)
	assert.Equal(t, "test-client", cfg.Name)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, 10, cfg.MaxReconnect)
	assert.Equal(t, 1*time.Second, cfg.ReconnectWait)
}

func TestMockClient_Publish(t *testing.T) {
	mock := &MockClient{}

	err := mock.Publish("test.subject", []byte("test data"), nats.Header{
		"X-Test": []string{"value"},
	})

	require.NoError(t, err)
	assert.Len(t, mock.PublishedMsgs, 1)
	assert.Equal(t, "test.subject", mock.PublishedMsgs[0].Subject)
	assert.Equal(t, []byte("test data"), mock.PublishedMsgs[0].Data)
	assert.Equal(t, "value", mock.PublishedMsgs[0].Headers.Get("X-Test"))
}

func TestMockClient_Subscribe(t *testing.T) {
	mock := &MockClient{}

	handler := func(msg *nats.Msg) {}
	sub, err := mock.Subscribe("test.subject", handler)

	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Len(t, mock.Subscriptions, 1)
	assert.Equal(t, "test.subject", mock.Subscriptions[0].Subject)
}

func TestMockClient_Request(t *testing.T) {
	t.Run("with response", func(t *testing.T) {
		mock := &MockClient{
			RequestResp: &nats.Msg{
				Subject: "reply.subject",
				Data:    []byte("response data"),
			},
		}

		resp, err := mock.Request("test.subject", []byte("request data"), time.Second)

		require.NoError(t, err)
		assert.Equal(t, []byte("response data"), resp.Data)
	})

	t.Run("with error", func(t *testing.T) {
		mock := &MockClient{
			RequestErr: assert.AnError,
		}

		_, err := mock.Request("test.subject", []byte("request data"), time.Second)

		require.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("default response", func(t *testing.T) {
		mock := &MockClient{}

		resp, err := mock.Request("test.subject", []byte("request data"), time.Second)

		require.NoError(t, err)
		assert.Equal(t, []byte("mock response"), resp.Data)
	})
}

func TestMockClient_HealthCheck(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		mock := &MockClient{}
		err := mock.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("with error", func(t *testing.T) {
		mock := &MockClient{
			HealthCheckErr: assert.AnError,
		}
		err := mock.HealthCheck()
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestMockClient_Drain(t *testing.T) {
	mock := &MockClient{}

	err := mock.Drain()

	assert.NoError(t, err)
}

func TestMockClient_Close(t *testing.T) {
	mock := &MockClient{}

	assert.False(t, mock.CloseCalled)

	mock.Close()

	assert.True(t, mock.CloseCalled)
}

func TestMockClient_GetPublishedMessages(t *testing.T) {
	mock := &MockClient{}

	// Add some published messages
	mock.Publish("subject1", []byte("data1"), nil)
	mock.Publish("subject2", []byte("data2"), nil)

	messages := mock.GetPublishedMessages()

	assert.Len(t, messages, 2)
	assert.Equal(t, "subject1", messages[0].Subject)
	assert.Equal(t, "subject2", messages[1].Subject)
}

func TestMockClient_GetSubscriptions(t *testing.T) {
	mock := &MockClient{}

	// Add some subscriptions
	mock.Subscribe("subject1", func(msg *nats.Msg) {})
	mock.Subscribe("subject2", func(msg *nats.Msg) {})

	subscriptions := mock.GetSubscriptions()

	assert.Len(t, subscriptions, 2)
	assert.Equal(t, "subject1", subscriptions[0].Subject)
	assert.Equal(t, "subject2", subscriptions[1].Subject)
}

func TestMockSubscription_Unsubscribe(t *testing.T) {
	sub := &MockSubscription{}

	err := sub.Unsubscribe()

	assert.NoError(t, err)
}

func TestMockClient_SubscribeToStream(t *testing.T) {
	mock := &MockClient{}

	handler := func(msg *nats.Msg) {}
	err := mock.SubscribeToStream(nil, "STREAM", "CONSUMER", "subject", handler)

	require.NoError(t, err)
	assert.Len(t, mock.StreamSubs, 1)
	assert.Equal(t, "STREAM", mock.StreamSubs[0].Stream)
	assert.Equal(t, "CONSUMER", mock.StreamSubs[0].Consumer)
	assert.Equal(t, "subject", mock.StreamSubs[0].Subject)
}

func TestMockClient_GetStreamSubscriptions(t *testing.T) {
	mock := &MockClient{}

	// Add some stream subscriptions
	mock.SubscribeToStream(nil, "STREAM1", "CONSUMER1", "subject1", func(msg *nats.Msg) {})
	mock.SubscribeToStream(nil, "STREAM2", "CONSUMER2", "subject2", func(msg *nats.Msg) {})

	subscriptions := mock.GetStreamSubscriptions()

	assert.Len(t, subscriptions, 2)
	assert.Equal(t, "STREAM1", subscriptions[0].Stream)
	assert.Equal(t, "CONSUMER1", subscriptions[0].Consumer)
	assert.Equal(t, "STREAM2", subscriptions[1].Stream)
	assert.Equal(t, "CONSUMER2", subscriptions[1].Consumer)
}

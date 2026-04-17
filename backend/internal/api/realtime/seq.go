package realtime

import (
	"sync"
	"time"
)

// SeqEntry represents a single entry in the sequence buffer.
type SeqEntry struct {
	Seq  uint64
	Data []byte
	Time time.Time
}

// SeqBufferConfig holds configuration for the sequence buffer.
type SeqBufferConfig struct {
	MaxSize int           // Maximum entries per topic
	MaxAge  time.Duration // Maximum age before entry expires
}

// DefaultSeqBufferConfig returns the default buffer configuration.
func DefaultSeqBufferConfig() SeqBufferConfig {
	return SeqBufferConfig{
		MaxSize: 1000,
		MaxAge:  10 * time.Minute,
	}
}

// SeqBuffer provides per-topic ring buffers for event replay.
type SeqBuffer struct {
	mu     sync.RWMutex
	buf    map[string][]SeqEntry
	config SeqBufferConfig
}

// NewSeqBuffer creates a new sequence buffer with the given configuration.
func NewSeqBuffer(config SeqBufferConfig) *SeqBuffer {
	if config.MaxSize <= 0 {
		config = DefaultSeqBufferConfig()
	}
	return &SeqBuffer{
		buf:    make(map[string][]SeqEntry),
		config: config,
	}
}

// Append adds an entry to the topic's ring buffer.
func (s *SeqBuffer) Append(topic string, seq uint64, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := SeqEntry{
		Seq:  seq,
		Data: data,
		Time: time.Now(),
	}

	// Get or create the topic buffer
	buffer := s.buf[topic]
	if buffer == nil {
		buffer = make([]SeqEntry, 0, s.config.MaxSize)
	}

	// Append to ring buffer
	buffer = append(buffer, entry)

	// Enforce max size (ring buffer behavior)
	if len(buffer) > s.config.MaxSize {
		// Remove oldest entries
		excess := len(buffer) - s.config.MaxSize
		buffer = buffer[excess:]
	}

	s.buf[topic] = buffer
}

// Replay returns events from the topic buffer since the given sequence number.
// Returns ErrBufferExceeded if the requested sequence is too old.
func (s *SeqBuffer) Replay(topic string, sinceSeq uint64) ([]SeqEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffer, ok := s.buf[topic]
	if !ok {
		return nil, nil
	}

	// Find starting position
	startIdx := -1
	for i, entry := range buffer {
		if entry.Seq > sinceSeq {
			startIdx = i
			break
		}
	}

	// Check for gap: if buffer is full and sinceSeq is in the range of lost entries
	// We lost entries [1, buffer[0].Seq-1] due to compaction
	// sinceSeq=0 means "replay all" (special case, no error)
	// sinceSeq >= buffer[0].Seq means the entry at sinceSeq is available
	// sinceSeq in (0, buffer[0].Seq) means we lost entries and can't replay
	if s.config.MaxSize > 0 && len(buffer) >= s.config.MaxSize && sinceSeq > 0 && sinceSeq < buffer[0].Seq {
		return nil, ErrBufferExceeded
	}

	if startIdx == -1 {
		return nil, nil
	}

	// Filter expired entries
	var result []SeqEntry
	for i := startIdx; i < len(buffer); i++ {
		entry := buffer[i]
		if s.config.MaxAge > 0 && time.Since(entry.Time) > s.config.MaxAge {
			continue
		}
		result = append(result, entry)
	}

	return result, nil
}

// GetLastSeq returns the last sequence number for a topic, or 0 if no events.
func (s *SeqBuffer) GetLastSeq(topic string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffer, ok := s.buf[topic]
	if !ok || len(buffer) == 0 {
		return 0
	}
	return buffer[len(buffer)-1].Seq
}

// Size returns the number of entries in the buffer for a topic.
func (s *SeqBuffer) Size(topic string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffer, ok := s.buf[topic]
	if !ok {
		return 0
	}
	return len(buffer)
}

// RemoveTopic removes all entries for a topic.
func (s *SeqBuffer) RemoveTopic(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.buf, topic)
}

// CleanExpired removes expired entries from all topic buffers.
func (s *SeqBuffer) CleanExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.MaxAge <= 0 {
		return 0
	}

	now := time.Now()
	removed := 0

	for topic, buffer := range s.buf {
		var newBuffer []SeqEntry
		for _, entry := range buffer {
			if now.Sub(entry.Time) <= s.config.MaxAge {
				newBuffer = append(newBuffer, entry)
			} else {
				removed++
			}
		}
		if len(newBuffer) == 0 {
			delete(s.buf, topic)
		} else {
			s.buf[topic] = newBuffer
		}
	}

	return removed
}

// Topics returns all topics currently in the buffer.
func (s *SeqBuffer) Topics() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics := make([]string, 0, len(s.buf))
	for topic := range s.buf {
		topics = append(topics, topic)
	}
	return topics
}

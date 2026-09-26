// Package chat holds per-session conversation history, in memory.
//
// In-memory is a deliberate choice for a single-instance free-tier deploy:
// history only needs to survive a conversation, not a restart, and a
// database table would add write traffic to every chat turn for no user-
// visible benefit. What it must not do is grow without bound - so the store
// is capped in sessions (least-recently-active eviction), in messages per
// session, in characters per stored message, and in session age.
package chat

import (
	"sync"
	"time"
)

const (
	maxMessagesPerSession = 20
	// maxStoredMessageChars caps what gets replayed into later prompts as
	// history. A long answer is useful once; re-sending all of it on every
	// following turn multiplies its LLM token cost across the conversation.
	maxStoredMessageChars = 1500
	truncationMarker      = " …[truncated]"
)

type Store struct {
	mu            sync.Mutex
	conversations map[string]*Conversation
	maxSessions   int
	ttl           time.Duration
}

// NewStore creates a store holding at most maxSessions conversations, each
// discarded once idle longer than ttl.
func NewStore(maxSessions int, ttl time.Duration) *Store {
	if maxSessions <= 0 {
		maxSessions = 1000
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{
		conversations: make(map[string]*Conversation),
		maxSessions:   maxSessions,
		ttl:           ttl,
	}
}

// AddMessage appends a message to sessionID's conversation, creating it if
// needed and evicting the least-recently-active session if that pushes the
// store past capacity.
func (s *Store) AddMessage(sessionID, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	conv, exists := s.conversations[sessionID]
	if !exists || s.expired(conv, now) {
		conv = &Conversation{SessionID: sessionID}
		s.conversations[sessionID] = conv
	}

	conv.Messages = append(conv.Messages, Message{Role: role, Content: truncate(content)})
	if len(conv.Messages) > maxMessagesPerSession {
		conv.Messages = conv.Messages[len(conv.Messages)-maxMessagesPerSession:]
	}
	conv.lastActive = now

	if len(s.conversations) > s.maxSessions {
		s.evictOldest()
	}
}

// RecentMessages returns up to limit of the session's latest messages, or
// nil for an unknown or expired session.
func (s *Store) RecentMessages(sessionID string, limit int) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[sessionID]
	if !exists || s.expired(conv, time.Now()) {
		return nil
	}

	msgs := conv.Messages
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}

	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

// PruneExpired removes every session idle past the TTL. Called
// periodically so abandoned sessions free their memory even if nobody
// adds a message that would trigger capacity eviction.
func (s *Store) PruneExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, conv := range s.conversations {
		if s.expired(conv, now) {
			delete(s.conversations, id)
		}
	}
}

// StartJanitor runs PruneExpired every interval until stop is closed.
func (s *Store) StartJanitor(interval time.Duration, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.PruneExpired()
			case <-stop:
				return
			}
		}
	}()
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conversations)
}

func (s *Store) expired(conv *Conversation, now time.Time) bool {
	return !conv.lastActive.IsZero() && now.Sub(conv.lastActive) > s.ttl
}

// evictOldest drops the least-recently-active session. Linear scan - fine
// at the few-thousand-session scale this is capped to, and it avoids
// maintaining a separate LRU list under the same lock.
func (s *Store) evictOldest() {
	var oldestID string
	var oldest time.Time
	for id, conv := range s.conversations {
		if oldestID == "" || conv.lastActive.Before(oldest) {
			oldestID, oldest = id, conv.lastActive
		}
	}
	if oldestID != "" {
		delete(s.conversations, oldestID)
	}
}

func truncate(content string) string {
	runes := []rune(content)
	if len(runes) <= maxStoredMessageChars {
		return content
	}
	return string(runes[:maxStoredMessageChars]) + truncationMarker
}

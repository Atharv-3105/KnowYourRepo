package chat

import (
	"strings"
	"testing"
	"time"
)

func TestStore_RecentMessagesReturnsLatest(t *testing.T) {
	s := NewStore(10, time.Hour)
	for i := 0; i < 5; i++ {
		s.AddMessage("sess", "user", string(rune('a'+i)))
	}

	got := s.RecentMessages("sess", 2)
	if len(got) != 2 || got[0].Content != "d" || got[1].Content != "e" {
		t.Errorf("expected the last 2 messages [d e], got %+v", got)
	}
}

func TestStore_CapsMessagesPerSession(t *testing.T) {
	s := NewStore(10, time.Hour)
	for i := 0; i < maxMessagesPerSession+15; i++ {
		s.AddMessage("sess", "user", "m")
	}

	if got := len(s.RecentMessages("sess", 1000)); got != maxMessagesPerSession {
		t.Errorf("expected %d retained messages, got %d", maxMessagesPerSession, got)
	}
}

// A long assistant answer is replayed into every later prompt as history -
// storing it whole would multiply its token cost across the conversation.
func TestStore_TruncatesLongMessageContent(t *testing.T) {
	s := NewStore(10, time.Hour)
	s.AddMessage("sess", "assistant", strings.Repeat("x", maxStoredMessageChars+500))

	got := s.RecentMessages("sess", 1)[0].Content
	if len([]rune(got)) > maxStoredMessageChars+len(truncationMarker) {
		t.Errorf("expected content truncated to ~%d chars, got %d", maxStoredMessageChars, len(got))
	}
	if !strings.HasSuffix(got, truncationMarker) {
		t.Errorf("expected truncated content to end with the truncation marker")
	}
}

// The session map previously grew without bound for the life of the
// process. Past capacity, the least-recently-active session is evicted.
func TestStore_EvictsLeastRecentlyActiveSessionAtCapacity(t *testing.T) {
	s := NewStore(2, time.Hour)
	s.AddMessage("a", "user", "1")
	time.Sleep(2 * time.Millisecond)
	s.AddMessage("b", "user", "1")
	time.Sleep(2 * time.Millisecond)
	s.AddMessage("a", "user", "2") // a is now more recent than b
	time.Sleep(2 * time.Millisecond)
	s.AddMessage("c", "user", "1") // over capacity - b should go

	if s.RecentMessages("b", 10) != nil {
		t.Error("expected least-recently-active session b to be evicted")
	}
	if len(s.RecentMessages("a", 10)) != 2 || len(s.RecentMessages("c", 10)) != 1 {
		t.Error("expected sessions a and c to survive")
	}
}

func TestStore_ExpiredSessionsAreDropped(t *testing.T) {
	s := NewStore(10, 5*time.Millisecond)
	s.AddMessage("old", "user", "hi")
	time.Sleep(15 * time.Millisecond)

	if got := s.RecentMessages("old", 10); got != nil {
		t.Errorf("expected an expired session to return no history, got %+v", got)
	}

	s.PruneExpired()
	if s.Len() != 0 {
		t.Errorf("expected PruneExpired to remove the expired session, %d remain", s.Len())
	}
}

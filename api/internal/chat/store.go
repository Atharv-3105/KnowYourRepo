//AN IN-MEMORY STORE FOR CHAT 
package chat 

import "sync"

type Store struct {
	mu        sync.RWMutex
	conversations   map[string]*Conversation
}

func NewStore() *Store{

	return &Store{
		conversations: make(map[string]*Conversation),
	}
}

//Function to retrieve a Saved Conversation
func(s *Store) GetConversation(sessionID  string) *Conversation{

	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, exists := s.conversations[sessionID]

	if !exists{
		return nil
	}

	return conv 
}

//Function to create conversation give a sessionID
func(s *Store) CreateConversation(sessionID string) *Conversation{

	s.mu.Lock()
	defer s.mu.Unlock()

	conv := &Conversation{
		SessionID: sessionID,
		Messages:   []Message{},
	}

	s.conversations[sessionID] = conv

	return conv 
}

func(s *Store) AddMessage(sessionID string, role string, content string) {

	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[sessionID]

	if !exists{

		conv = &Conversation{
			SessionID: sessionID,
		}

		s.conversations[sessionID] = conv
	}

	conv.Messages = append(conv.Messages, 
					Message{
						Role: role, 
						Content: content,
					})

	//Memory limit for only saving the latest 20 messages if number of messages are over the limit
	if len(conv.Messages) > 20 {
		conv.Messages = conv.Messages[len(conv.Messages) - 20:]
	}
}


//Function to send only the latest/recent history
func(s *Store) RecentMessages(sessionID string, limit int) []Message{
		s.mu.RLock()
		defer s.mu.RUnlock()

		conv, exists := s.conversations[sessionID]

		if !exists{
			return nil
		}

		msgs := conv.Messages

		if len(msgs) <= limit{
			return msgs
		}

		return msgs[len(msgs) - limit:]
}   
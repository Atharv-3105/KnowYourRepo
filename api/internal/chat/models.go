package chat

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Conversation struct {
	SessionID  string    `json:"session_id"`
	Messages   []Message `json:"messages"`
	lastActive time.Time
}

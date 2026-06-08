package chat

import "strings"

func BuildConversationContext(messages []Message) string {

	var b strings.Builder

	for _, msg := range messages{

		b.WriteString(msg.Role)
		b.WriteString(": ")
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}

	return b.String()
}
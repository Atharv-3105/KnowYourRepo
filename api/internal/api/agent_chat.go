package api

import (
	"net/http"

	"github.com/atharva-3105/KnowYourRepo/internal/chat"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/gin-gonic/gin"
)

type AgentChatResponse struct {
	Answer     string   `json:"answer"`
	Tools      []string `json:"tools_used"`
	Refreshing bool     `json:"refreshing"`
	Sources    []Source `json:"sources"`
}

// Source is one piece of retrieved context that fed the LLM's answer, carried
// through so the frontend can render a clickable file:line citation. FilePath
// comes straight off retrieval.RetrievalResult; StartLine/EndLine come from
// its loosely-typed Metadata map (round-tripped through the sidecar's JSON,
// so numbers land as float64) and default to 0 when absent or malformed
// rather than erroring - a citation missing a line number should still show
// the file, not fail the whole response.
type Source struct {
	Symbol    string `json:"symbol"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func buildSources(results []retrieval.RetrievalResult) []Source {

	sources := make([]Source, 0, len(results))

	for _, r := range results {

		sources = append(sources, Source{
			Symbol:    r.Symbol,
			FilePath:  r.FilePath,
			StartLine: metadataInt(r.Metadata, "start_line"),
			EndLine:   metadataInt(r.Metadata, "end_line"),
		})
	}

	return sources
}

// metadataInt safely reads an int out of a metadata map whose values arrive
// as interface{} (typically float64, since they crossed a JSON boundary via
// the sidecar's response). Returns 0 for a missing key or an unexpected type
// instead of panicking - this map is externally-sourced, loosely-typed data.
func metadataInt(metadata map[string]interface{}, key string) int {

	raw, ok := metadata[key]
	if !ok {
		return 0
	}

	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func (h *RepoHandler) AgentChat(c *gin.Context) {

	var req ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	history := h.chatStore.RecentMessages(req.SessionID, 6)
	conversationContext := chat.BuildConversationContext(history)

	h.logger.Info("agent_conversation_loaded", "session_id", req.SessionID, "messages", len(history))

	h.chatStore.AddMessage(req.SessionID, "user", req.Question)

	answer, plan, refreshing, results, err := h.agentService.Answer(
		c.Request.Context(),
		req.RepoID,
		req.Question,
		conversationContext,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.chatStore.AddMessage(req.SessionID, "assistant", answer)
	h.logger.Info("agent_chat_message_saved", "session_id", req.SessionID, "role", "assistant")

	toolNames := make([]string, 0, len(plan))
	for _, t := range plan {
		toolNames = append(toolNames, string(t))
	}

	c.JSON(http.StatusOK, AgentChatResponse{
		Answer:     answer,
		Tools:      toolNames,
		Refreshing: refreshing,
		Sources:    buildSources(results),
	})
}
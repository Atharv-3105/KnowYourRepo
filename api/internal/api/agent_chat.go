package api

import (
	"net/http"

	"github.com/atharva-3105/KnowYourRepo/internal/chat"
	"github.com/gin-gonic/gin"
)

type AgentChatResponse struct{
	Answer string 	`json:"answer"`
	Tools  []string `json:"tools_used"`
}

func(h *RepoHandler) AgentChat(c *gin.Context){

	var req ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return 
	}

	//Get the History and build-up the Conversation Context
	history := h.chatStore.RecentMessages(req.SessionID, 6)
	conversationContext := chat.BuildConversationContext(history)

	h.logger.Info("agent_conversation_loaded", "session_id", req.SessionID, "messages", len(history))

	h.chatStore.AddMessage(req.SessionID, "user", req.Question)

	answer, plan, err := h.agentService.Answer(c.Request.Context(), req.RepoID, req.Question, conversationContext)

	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
	}

	h.chatStore.AddMessage(req.SessionID, "assistant", answer)
	h.logger.Info("agent_chat_message_saved", "session_id", req.SessionID, "role", "assistant")

	toolNames := make([]string, 0, len(plan))
	for _, t := range plan{

		toolNames = append(toolNames, string(t))
	}

	c.JSON(http.StatusOK, AgentChatResponse{
		Answer: answer,
		Tools: toolNames,
	})
}
package api

import (
	"fmt"
	"net/http"

	"github.com/atharva-3105/KnowYourRepo/internal/chat"
	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	RepoID       string 	 `json:"repo_id"`
	Question     string      `json:"question"`
	SessionID    string      `json:"session_id"`
}

type ChatResponse struct {
	Answer     string       `json:"answer"`
}

func (h *RepoHandler) Chat (c *gin.Context) {

	var req ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return 
	}

	//Load History before Retrieval
	history := h.chatStore.RecentMessages(req.SessionID, 6)
	conversationContext := chat.BuildConversationContext(history)

	h.logger.Info("conversation_loaded", "session_id", req.SessionID, "messages", len(history))

	query := req.Question
 
	// if len(history) > 0 {

	// 	query = conversationContext + "\nCurrent Question:\n" + req.Question
	// }

	//Store User Message
	h.chatStore.AddMessage(req.SessionID, "user", req.Question)
	//log successfully saved
	h.logger.Info("chat_message_saved", "session_id", req.SessionID, "role", "user")


	//Hybrid Retrieval
	results, err := h.hybridRetriever.Search(
		c.Request.Context(),
		req.RepoID,
		query,
	)

	if err != nil {
		
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return 
	}

	fmt.Println("Retrieved Results: ", len(results))

	//Perform RAG
	answer, err := h.ragService.AnswerQuestion(
		c.Request.Context(),
		query,
		conversationContext,
		results,
	)
	if err != nil {

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)

		return 
	}

	//store assistant message 
	h.chatStore.AddMessage(req.SessionID, "assistant", answer)
	h.logger.Info("chat_message_saved", "session_id", req.SessionID, "role", "assistant")
	

	c.JSON(
		http.StatusOK,
		ChatResponse{
			Answer: answer,
		},
	)

}
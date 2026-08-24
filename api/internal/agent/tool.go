package agent 

import "context"


//This interface will act as the boundary between the agent and our system's determinisitic repository-understanding capabilities
//The agent orchestrates tools rather than reimplementing them.
type Tool interface {
	Name()  ToolName 
	Execute(ctx context.Context, req ToolRequest) (ToolResult, error)
}
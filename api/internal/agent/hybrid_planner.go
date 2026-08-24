package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
)

//this bounds how long the hybrid planner waits for the classification path before giving up & falling back to
//the deterministic planner.
const classifyTimeout = 5 * time.Second 

//Our HybridPlanner tries LLM-Based classification first(via the sidecar's /classify endpoint, which has multi-provider failover)
//and falls back to the deterministic Planner on any error, timeout or invalid response.
//The executor will never know which path produced the plan
type HybridPlanner struct {
	sidecar   *sidecar.Client 
	fallback  *Planner 
	logger    *slog.Logger
}

func NewHybridPlanner(sidecarClient *sidecar.Client, fallback *Planner, logger *slog.Logger) *HybridPlanner {
	return &HybridPlanner{
		sidecar: sidecarClient,
		fallback: fallback,
		logger: logger, 
	}
}

func (p *HybridPlanner) Plan(ctx context.Context, query, history string) []ToolName {
	plan, err := p.planWithLLM(ctx, query, history)
	if err == nil && len(plan) > 0 {
		p.logger.Info("agent_plan_source", "source", "llm", "tools", plan)
		return plan 
	}

	if err != nil {
		if errors.Is(err, sidecar.ErrClassificationUnavailable) {
			p.logger.Warn("agent_plan_llm_unavailable", "reason", "all_providers_exhausted")
		}else{
			p.logger.Warn("agent_plan_llm_failed", "error", err)
		}
	}else{
		p.logger.Warn("agent_plan_llm_empty_response")
	}

	fallbackPlan := p.fallback.Plan(query, history)

	p.logger.Info("agent_plan_source", "source", "deterministic", "tools", fallbackPlan)

	return fallbackPlan
}

func(p *HybridPlanner) planWithLLM(ctx context.Context, query, history string) ([]ToolName, error) {

	//define custom-timeout for planner requests
	classifyCtx, cancel := context.WithTimeout(ctx, classifyTimeout)
	defer cancel()

	resp, err := p.sidecar.Classify(classifyCtx, sidecar.ClassifyRequest{
		Question: query,
		History: history,
	})

	if err != nil{
		return nil, err 
	}

	return toValidToolNames(resp.Tools), nil 
}


//toValidToolNames filters the sidecar's response down to tool names that are actually known about, so an unexpected tool name from the LLM never reaches the executor agent
func toValidToolName(names []string)[]ToolName {

	var output []ToolName

	for _, n := range names {
		switch ToolName(n) {
		case ToolSemantic, ToolArchitecture, ToolGraph, ToolMemory:
			output = append(output, ToolName(n))
		}
	}

	return dedupeToolNames(output)
}
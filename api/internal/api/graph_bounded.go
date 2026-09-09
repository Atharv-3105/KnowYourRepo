package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

const (
	defaultGraphDepth = 2
	maxGraphDepth     = 5
	maxGraphNodes     = 150
	maxGraphEdgeRows  = 1000
)

type GraphNode struct {
	Symbol   string `json:"symbol"`
	FilePath string `json:"file_path,omitempty"`
}

type GraphEdgeResponse struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
}

type BoundedGraphResponse struct {
	RepoID     string              `json:"repo_id"`
	RootSymbol string              `json:"root_symbol"`
	Depth      int                 `json:"depth"`
	Truncated  bool                `json:"truncated"`
	Nodes      []GraphNode         `json:"nodes"`
	Edges      []GraphEdgeResponse `json:"edges"`
}

// GetBoundedCallGraph handles GET /graph/:repoID?symbol=X&depth=N - unlike the
// legacy GET /graph (unscoped, unbounded, dumps every edge across every repo),
// this returns a depth-capped, node-capped subgraph anchored at one symbol,
// scoped to one repository, sized for actually being rendered in a UI.
func (h *RepoHandler) GetBoundedCallGraph(c *gin.Context) {

	repoID := c.Param("repoID")

	if !h.requireRepoExists(c, repoID) {
		return
	}

	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol query param is required"})
		return
	}

	depth := defaultGraphDepth
	if raw := c.Query("depth"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "depth must be a positive integer"})
			return
		}
		depth = parsed
	}
	if depth > maxGraphDepth {
		depth = maxGraphDepth
	}

	edges, err := h.store.GetBoundedCallGraph(c.Request.Context(), repoID, symbol, depth, maxGraphEdgeRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := buildBoundedGraphResponse(repoID, symbol, depth, edges)

	c.JSON(http.StatusOK, response)
}

// buildBoundedGraphResponse dedupes nodes/edges out of the raw edge rows and enforces
// the node cap in application code (the SQL LIMIT bounds edge *rows*, which isn't the
// same as bounding distinct *nodes* - a single hot node can appear in hundreds of rows).
func buildBoundedGraphResponse(repoID, rootSymbol string, depth int, edges []store.CallGraphEdge) BoundedGraphResponse {

	nodeFilePath := make(map[string]string)
	seenNode := make(map[string]struct{})
	seenEdge := make(map[[2]string]struct{})

	var nodeOrder []string
	var responseEdges []GraphEdgeResponse
	truncated := len(edges) >= maxGraphEdgeRows

	addNode := func(symbol, filePath string) bool {

		if _, ok := seenNode[symbol]; !ok {
			if len(seenNode) >= maxGraphNodes {
				return false
			}
			seenNode[symbol] = struct{}{}
			nodeOrder = append(nodeOrder, symbol)
		}

		if filePath != "" {
			if _, ok := nodeFilePath[symbol]; !ok {
				nodeFilePath[symbol] = filePath
			}
		}

		return true
	}

	addNode(rootSymbol, "")

	for _, e := range edges {

		if !addNode(e.Caller, e.CallerFilePath) {
			truncated = true
			break
		}
		// A callee's own file path is only known if it later shows up as a caller
		// elsewhere in this same traversal (call_edges has no callee_file_path column).
		if !addNode(e.Callee, "") {
			truncated = true
			break
		}

		key := [2]string{e.Caller, e.Callee}
		if _, ok := seenEdge[key]; ok {
			continue
		}
		seenEdge[key] = struct{}{}

		responseEdges = append(responseEdges, GraphEdgeResponse{Caller: e.Caller, Callee: e.Callee})
	}

	nodes := make([]GraphNode, 0, len(nodeOrder))
	for _, symbol := range nodeOrder {
		nodes = append(nodes, GraphNode{Symbol: symbol, FilePath: nodeFilePath[symbol]})
	}

	return BoundedGraphResponse{
		RepoID:     repoID,
		RootSymbol: rootSymbol,
		Depth:      depth,
		Truncated:  truncated,
		Nodes:      nodes,
		Edges:      responseEdges,
	}
}

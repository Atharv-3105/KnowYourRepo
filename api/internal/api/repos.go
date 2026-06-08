package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	// "os"
	"path/filepath"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/chat"
	"github.com/atharva-3105/KnowYourRepo/internal/chunk"
	"github.com/atharva-3105/KnowYourRepo/internal/contextbuilder"
	"github.com/atharva-3105/KnowYourRepo/internal/graph"
	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
	"github.com/atharva-3105/KnowYourRepo/internal/rag"
	"github.com/atharva-3105/KnowYourRepo/internal/representation"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
	"github.com/gin-gonic/gin"
)

type RepoHandler struct {
	logger 		*slog.Logger 
	store  		*store.Store
	sidecar  	*sidecar.Client
	parser   	*ingestion.TreeSitterParser
	extractor  	*graph.Extractor
	cloner      *ingestion.Cloner
	walker      *ingestion.Walker
	// graphRetriever   *retrieval.GraphRetriever
	hybridRetriever  *retrieval.HybridRetriever
	contextBuilder   *contextbuilder.Builder
	ragService 		*rag.Service
	chatStore       *chat.Store
}

func NewRepoHandler(
	logger   *slog.Logger,
	store    *store.Store,
	sidecar  *sidecar.Client,
) *RepoHandler {

	builder := contextbuilder.NewBuilder(logger)
	return &RepoHandler{
		logger:  logger,
		store:   store,
		sidecar:  sidecar,
		parser:   ingestion.NewParser(logger),
		extractor: graph.NewExtractor(logger),
		cloner:    ingestion.NewCloner(logger),
		walker:    ingestion.NewWalker(logger),
		// graphRetriever:   retrieval.NewGraphRetriever(store),
		hybridRetriever:  retrieval.NewHybridRetriever(store, sidecar, logger),
		contextBuilder:   builder,
		ragService: 	  rag.NewService(builder, sidecar, logger),
		chatStore:		  chat.NewStore(),
	}
}


func (h *RepoHandler) CreateRepo(c *gin.Context) {

	var req CreateRepoRequest

	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx := context.Background()
	repoID := generateRepoDirName()

	repoDir := filepath.Join(
		"..",
		"data",
		"repos",
		repoID,
	)

	//Insert the Repository with repo_id into the repositories tables
	err := h.store.InsertRepository(ctx, store.Repository{
					ID:		repoID,
					RepoURL:   req.RepoURL,
	})

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)

		return 
	}

	h.logger.Info("starting repo ingestion", "repo_url", req.RepoURL)

	//Clone the Repository
	if err := h.cloner.CloneRepo(ctx,req.RepoURL,repoDir); err != nil {
		h.logger.Error("clone_failed", "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
	}

	//Declare RepoIR as JSON
	repoIR := representation.Repository{
		Name:	filepath.Base(repoDir),
	}


	//Get the files of the Repository
	files, err := h.walker.WalkRepo(ctx,repoDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
	}

	h.logger.Info("found files", "count", len(files))

	var embedItems []sidecar.EmbedBatchItem

	//Process the files
	for _, file := range files {

		h.logger.Debug("processing_files", "path", file.Path)

		//Insert file into the DB
		fileID, err := h.store.InsertFile(ctx, file.Path, file.Language)
		if err != nil {
			h.logger.Error("failed to insert file", "error", err)
			continue
		}

		//Parse AST
		parseResult, err := h.parser.ParseFile(ctx, file.Path, file.Language)
		if err != nil{
			h.logger.Warn("parsing failed", "path", file.Path, "error", err)
			continue
		}

		chunks := chunk.ExtractFunctionChunks(parseResult.Root, parseResult.Source, file.Language, file.Path)

		var finalChunks []chunk.Chunk

		for _, ch := range chunks {

			splitChunks := chunk.SplitChunk(ch)

			finalChunks = append(finalChunks, splitChunks...)
		}
		
		//Convert file into IR file object
		irFile := representation.NewFile(file.Path, file.Language)

		//Extract the Symbols
		symbols, err := h.extractor.ExtractSymbols(parseResult.Root, parseResult.Source, file.Language)
		if err != nil{
			h.logger.Warn("symbol extraction failed", "path", file.Path, "error", err)
			continue
		}

		//Call Graph Extraction
		var edges []graph.CallEdge

		switch file.Language {

		case "go":
				edges = graph.ExtractGoCallGraph(parseResult.Root, parseResult.Source)

		case "python":
				edges = graph.ExtractPythonCallGraph(parseResult.Root, parseResult.Source)

		case "javascript", "typescript":
				edges = graph.ExtractJSCallGraph(parseResult.Root, parseResult.Source)
		}
		

		for _, edge := range edges {

			err := h.store.InsertCallEdge(ctx, 
				store.CallEdge{
					CallerSymbol: edge.Caller, 
					CallerFilePath: file.Path,
					CalleeSymbol: edge.Callee,
				})
			
			if err != nil {

				h.logger.Error("failed to store call edges", "caller", edge.Caller, "callee", edge.Callee, "error", err)
			}

			h.logger.Info(
				"call_edge_inserted",
				"caller", edge.Caller,
				"caller_file_path", file.Path,
				"callee", edge.Callee,
			)

			irFile.Calls = append(irFile.Calls, 
			  			   representation.CallEdge{
								Caller:    edge.Caller,
								Callee:    edge.Callee,
						   })
		}

		//Store Symbolss
		for _, sym := range symbols{

			symbolID, err := h.store.InsertSymbol(ctx,
				store.Symbol{
					FileID: fileID,
					Name:  sym.Name,
					Type:  sym.Type,
					StartLine: sym.StartLine,
					EndLine:  sym.EndLine,
				},
			)

			if err != nil{
				h.logger.Error("failed to insert symbol", "error", err)
				continue
			}

			//Append items in IR 
			irFile.Symbols = append(irFile.Symbols, 
							 representation.Symbol{
								ID: 	fmt.Sprintf("%d", symbolID),
								Name: 	sym.Name,
								Kind:	sym.Type,
								Language: file.Language,
								FilePath: file.Path,
								StartLine:  sym.StartLine,
								EndLine:   sym.EndLine,
							 })

			// embedItems = append(embedItems, 
			// 					sidecar.EmbedBatchItem{
			// 						ID:  fmt.Sprintf("%d", symbolID),
			// 						Text:  fmt.Sprintf("%s %s", sym.Type, sym.Name),
			// 						Metadata: map[string]interface{}{
			// 							"file_path": file.Path,
			// 							"language": file.Language,
			// 							"type": sym.Type,
			// 						},
			// 					})
		}

		for idx, ch := range finalChunks {

			embedItems = append(embedItems, 
			  			sidecar.EmbedBatchItem{
							ID:   fmt.Sprintf("%s_part_%d", ch.ID, idx),
							Text:  ch.Content,
							Metadata: map[string]interface{} {
								"repo_id": repoID,
								"file_path": ch.FilePath,
								"language": ch.Language,
								"symbol":   ch.SymbolName,
								"start_line": ch.StartLine,
								"end_line": ch.EndLine,
								"chunk_size": len(ch.Content),
							},
						})
		}

		repoIR.Files = append(repoIR.Files, irFile)
	}

	h.logger.Info("sending batch embeddings", "count", len(embedItems))

	//Send 1 Batch Embedding Request only after processing all files
	err = h.sidecar.Embed(ctx, sidecar.EmbedBatchRequest{Items:embedItems})

	if err != nil {
		h.logger.Error("batch embedding failed", "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})

		return 
	}

	//Save IR File Representation
	irPath := filepath.Join(repoDir, "repository_ir.json")

	err = representation.SaveRepository(repoIR, irPath)

	if err != nil {
		h.logger.Error("failed to save repository IR", "error", err)
	}

	c.JSON(http.StatusOK, CreateRepoResponse{
				Success: true,
				Message: "repository ingested successfully",
				RepoID: repoID,
	})
}

func generateRepoDirName() string {
	return fmt.Sprintf("repo_%d", time.Now().UnixNano())
}
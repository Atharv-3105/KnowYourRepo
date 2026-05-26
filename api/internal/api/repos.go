package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	// "os"
	"path/filepath"
	"time"

	"github.com/atharva-3105/KnowYourRepo/internal/graph"
	"github.com/atharva-3105/KnowYourRepo/internal/ingestion"
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
}

func NewRepoHandler(
	logger   *slog.Logger,
	store    *store.Store,
	sidecar  *sidecar.Client,
) *RepoHandler {

	return &RepoHandler{
		logger:  logger,
		store:   store,
		sidecar:  sidecar,
		parser:   ingestion.NewParser(logger),
		extractor: graph.NewExtractor(logger),
		cloner:    ingestion.NewCloner(logger),
		walker:    ingestion.NewWalker(logger),
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

	repoDir := filepath.Join(
		"..",
		"data",
		"repos",
		generateRepoDirName(),
	)

	h.logger.Info("starting repo ingestion", "repo_url", req.RepoURL)

	//Clone the Repository
	if err := h.cloner.CloneRepo(ctx,req.RepoURL,repoDir); err != nil {
		h.logger.Error("clone_failed", "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
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

		//Extract the Symbols
		symbols, err := h.extractor.ExtractSymbols(parseResult.Root, parseResult.Source, file.Language)
		if err != nil{
			h.logger.Warn("symbol extraction failed", "path", file.Path, "error", err)
			continue
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

			embedItems = append(embedItems, 
								sidecar.EmbedBatchItem{
									ID:  fmt.Sprintf("%d", symbolID),
									Text:  fmt.Sprintf("%s %s", sym.Type, sym.Name),
									Metadata: map[string]interface{}{
										"file_path": file.Path,
										"language": file.Language,
										"type": sym.Type,
									},
								})
		}
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


	c.JSON(http.StatusOK, CreateRepoResponse{
				Success: true,
				Message: "repository ingested successfully",
	})
}

func generateRepoDirName() string {
	return fmt.Sprintf("repo_%d", time.Now().UnixNano())
}
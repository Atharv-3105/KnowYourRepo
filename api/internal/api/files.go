package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxServableFileBytes caps what GetFileContent will read into memory and
// return as JSON - large vendored blobs (minified bundles, lockfiles) are a
// real, common thing to find in a real repo, and this endpoint has no
// business trying to serve or highlight megabytes of them.
const maxServableFileBytes = 2 << 20 // 2MB

type FileContentResponse struct {
	RepoID  string `json:"repo_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// extractSnippet returns the lines [startLine, endLine] (1-indexed,
// inclusive) of content. When both are 0 (no range requested), returns
// content unchanged. Out-of-bounds values are clamped rather than treated
// as errors - the caller's line numbers come from the symbol index at
// ingestion time and the file on disk can drift slightly since (a trailing
// blank line, a line ending change), so a range that's slightly past the
// end of the file degrades to whatever's actually there instead of failing
// the whole request. A start past the actual end of the file returns "" -
// there's genuinely nothing there to show.
func extractSnippet(content string, startLine, endLine int) string {
	if startLine <= 0 && endLine <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")

	if startLine < 1 {
		startLine = 1
	}
	if startLine > len(lines) {
		return ""
	}
	if endLine < startLine {
		endLine = startLine
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	return strings.Join(lines[startLine-1:endLine], "\n")
}

// GetFileContent handles GET /files/:repoID?path=<repo-relative path> -
// serves a single file's raw content from the already-on-disk clone at
// data/repos/<repoID>/ (kept around after ingestion specifically for
// future file-level retrieval like this). path is fully user-supplied, so
// it's rejected upfront if absolute (e.g. "C:/Windows/win.ini" or
// "/etc/passwd"), and the resolved path is independently verified to stay
// inside the repo's own clone directory before anything is opened -
// two checks, since an absolute-looking segment embedded mid-path can
// still survive filepath.Join without being caught by only one of them.
// Filesystem errors are logged server-side but never returned verbatim to
// the client - a raw os.Stat/os.ReadFile error can contain this server's
// absolute directory layout.
func (h *RepoHandler) GetFileContent(c *gin.Context) {

	repoID := c.Param("repoID")

	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path query param is required"})
		return
	}

	if filepath.IsAbs(relPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must be relative to the repository root"})
		return
	}

	h.logger.Info("file_content_requested", "repo_id", repoID, "path", relPath)

	repo, err := h.store.GetRepositoryByID(c.Request.Context(), repoID)
	if err != nil {
		h.logger.Error("file_content_lookup_failed", "repo_id", repoID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up repository"})
		return
	}
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	repoDir, err := filepath.Abs(filepath.Join("..", "data", "repos", repoID))
	if err != nil {
		h.logger.Error("file_content_repo_dir_resolve_failed", "repo_id", repoID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve repository path"})
		return
	}

	candidate, err := filepath.Abs(filepath.Join(repoDir, relPath))
	if err != nil {
		h.logger.Error("file_content_path_resolve_failed", "repo_id", repoID, "path", relPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve file path"})
		return
	}

	// The resolved path must be repoDir itself or a real descendant of it -
	// a prefix-string check alone would wrongly let "data/repos/repo_1x"
	// pass for repoDir "data/repos/repo_1", so the boundary is enforced
	// with an explicit separator.
	if candidate != repoDir && !strings.HasPrefix(candidate, repoDir+string(filepath.Separator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path escapes the repository directory"})
		return
	}

	info, err := os.Stat(candidate)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if err != nil {
		h.logger.Error("file_content_stat_failed", "repo_id", repoID, "path", relPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path refers to a directory, not a file"})
		return
	}
	if info.Size() > maxServableFileBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file is too large to display"})
		return
	}

	content, err := os.ReadFile(candidate)
	if err != nil {
		h.logger.Error("file_content_read_failed", "repo_id", repoID, "path", relPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// start_line/end_line are optional - when present, only that range is
	// returned instead of the whole file, for callers that just want a
	// small preview (e.g. the reading-path/concepts snippets on Overview)
	// rather than the full source.
	startLine, endLine := 0, 0
	if raw := c.Query("start_line"); raw != "" {
		startLine, err = strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_line must be an integer"})
			return
		}
	}
	if raw := c.Query("end_line"); raw != "" {
		endLine, err = strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end_line must be an integer"})
			return
		}
	}

	responseContent := extractSnippet(string(content), startLine, endLine)

	h.logger.Info("file_content_served", "repo_id", repoID, "path", relPath, "bytes", len(responseContent))

	c.JSON(http.StatusOK, FileContentResponse{
		RepoID:  repoID,
		Path:    relPath,
		Content: responseContent,
	})
}

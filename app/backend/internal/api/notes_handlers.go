package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	notesArticleCacheMu sync.Mutex
	notesBundleOnce     sync.Once
	notesBundle         map[string]string
	notesIDPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,80}$`)
)

func readNotesFile(name string) ([]byte, error) {
	candidates := []string{
		filepath.Join("data", name),
		filepath.Join("app", "backend", "data", name),
	}
	var last error
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return b, nil
		}
		last = err
	}
	return nil, last
}

func notesCachePath(id string) string {
	for _, root := range []string{filepath.Join("data", "notes-cache"), filepath.Join("app", "backend", "data", "notes-cache")} {
		if err := os.MkdirAll(root, 0o755); err == nil {
			return filepath.Join(root, id+".json")
		}
	}
	return filepath.Join("data", "notes-cache", id+".json")
}

func loadNotesBundle() map[string]string {
	notesBundleOnce.Do(func() {
		notesBundle = map[string]string{}
		raw, err := readNotesFile("notes-articles.json")
		if err != nil {
			return
		}
		_ = json.Unmarshal(raw, &notesBundle)
	})
	return notesBundle
}

// GetNotesTOC handles GET /api/notes/toc
func (h *Handler) GetNotesTOC(c *gin.Context) {
	raw, err := readNotesFile("notes-toc.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "notes toc not found"})
		return
	}
	var toc any
	if err := json.Unmarshal(raw, &toc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid notes toc", "detail": err.Error()})
		return
	}
	c.Header("X-Data-Source", "local-notes")
	c.JSON(http.StatusOK, toc)
}

// GetNotesArticle handles GET /api/notes/art?id= (local bundle/cache only).
func (h *Handler) GetNotesArticle(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" || !notesIDPattern.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if md, ok := loadNotesBundle()[id]; ok && strings.TrimSpace(md) != "" {
		c.Header("X-Data-Source", "local-notes")
		c.JSON(http.StatusOK, gin.H{"md": md, "id": id, "source": "local"})
		return
	}

	if md, ok := loadCachedNotesArticle(id); ok {
		c.Header("X-Data-Source", "local-notes-cache")
		c.JSON(http.StatusOK, gin.H{"md": md, "id": id, "source": "cache"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "article not found in local notes bundle", "id": id})
}

func loadCachedNotesArticle(id string) (string, bool) {
	notesArticleCacheMu.Lock()
	defer notesArticleCacheMu.Unlock()

	path := notesCachePath(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var payload struct {
		MD string `json:"md"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || strings.TrimSpace(payload.MD) == "" {
		return "", false
	}
	return payload.MD, true
}

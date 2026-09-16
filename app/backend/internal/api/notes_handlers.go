package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	notesIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,80}$`)
)

func readNotesFileWithInfo(name string) ([]byte, string, os.FileInfo, error) {
	candidates := []string{
		filepath.Join("data", name),
		filepath.Join("app", "backend", "data", name),
	}
	var last error
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			info, statErr := os.Stat(p)
			if statErr != nil {
				last = statErr
				continue
			}
			return b, p, info, nil
		}
		last = err
	}
	return nil, "", nil, last
}

func loadNotesBundle() (map[string]string, string) {
	bundle := map[string]string{}
	raw, _, info, err := readNotesFileWithInfo("notes-articles.json")
	if err != nil || json.Unmarshal(raw, &bundle) != nil {
		return bundle, "unknown"
	}
	return bundle, notesContentDataTime(raw, info)
}

// GetNotesTOC handles GET /api/notes/toc
func (h *Handler) GetNotesTOC(c *gin.Context) {
	raw, _, info, err := readNotesFileWithInfo("notes-toc.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "notes toc not found"})
		return
	}
	var toc any
	if err := json.Unmarshal(raw, &toc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid notes toc", "detail": err.Error()})
		return
	}
	dataTime := notesContentDataTime(raw, info)
	if object, ok := toc.(map[string]interface{}); ok {
		object["updatedAt"] = dataTime
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      "local-notes",
		DataTime:    dataTime,
		RefreshedAt: dataTime,
		Refreshable: false,
	})
	c.JSON(http.StatusOK, toc)
}

// GetNotesArticle handles GET /api/notes/art?id= (local bundle/cache only).
func (h *Handler) GetNotesArticle(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" || !notesIDPattern.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	bundle, bundleDataTime := loadNotesBundle()
	if md, ok := bundle[id]; ok && strings.TrimSpace(md) != "" {
		setDataFreshness(c, dataFreshnessMeta{
			Source:      "local-notes",
			DataTime:    bundleDataTime,
			RefreshedAt: bundleDataTime,
			Refreshable: false,
		})
		c.JSON(http.StatusOK, gin.H{"md": md, "id": id, "source": "local", "updatedAt": bundleDataTime})
		return
	}

	if md, dataTime, ok := loadCachedNotesArticle(id); ok {
		setDataFreshness(c, dataFreshnessMeta{
			Source:      "local-notes-cache",
			DataTime:    dataTime,
			RefreshedAt: dataTime,
			Refreshable: false,
		})
		c.JSON(http.StatusOK, gin.H{"md": md, "id": id, "source": "cache", "updatedAt": dataTime})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "article not found in local notes bundle", "id": id})
}

// GetNotesSearch handles GET /api/notes/search?q= and searches the local article bodies.
func (h *Handler) GetNotesSearch(c *gin.Context) {
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if query == "" || len([]rune(query)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q must contain 1 to 100 characters"})
		return
	}

	bundle, dataTime := loadNotesBundle()
	if len(bundle) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "notes search index unavailable"})
		return
	}
	type searchResult struct {
		ID      string `json:"id"`
		Snippet string `json:"snippet"`
	}
	results := make([]searchResult, 0, 24)
	ids := make([]string, 0, len(bundle))
	for id := range bundle {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		markdown := bundle[id]
		if !strings.Contains(strings.ToLower(markdown), query) {
			continue
		}
		runes := []rune(strings.Join(strings.Fields(markdown), " "))
		if len(runes) > 140 {
			runes = runes[:140]
		}
		results = append(results, searchResult{ID: id, Snippet: string(runes)})
		if len(results) == 24 {
			break
		}
	}
	setDataFreshness(c, dataFreshnessMeta{Source: "local-notes", DataTime: dataTime, RefreshedAt: dataTime, Refreshable: false})
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func loadCachedNotesArticle(id string) (string, string, bool) {
	for _, root := range []string{filepath.Join("data", "notes-cache"), filepath.Join("app", "backend", "data", "notes-cache")} {
		path := filepath.Join(root, id+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload struct {
			MD        string `json:"md"`
			UpdatedAt string `json:"updatedAt"`
		}
		if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.MD) != "" {
			dataTime := strings.TrimSpace(payload.UpdatedAt)
			if _, ok := parseLooseDataTime(dataTime); !ok {
				if info, statErr := os.Stat(path); statErr == nil {
					dataTime = info.ModTime().UTC().Format(time.RFC3339)
				}
			}
			return payload.MD, dataTime, true
		}
	}
	return "", "", false
}

func notesContentDataTime(raw []byte, info os.FileInfo) string {
	var metadata struct {
		UpdatedAt interface{} `json:"updatedAt"`
	}
	if json.Unmarshal(raw, &metadata) == nil {
		var candidate string
		switch value := metadata.UpdatedAt.(type) {
		case float64:
			candidate = time.UnixMilli(int64(value)).UTC().Format(time.RFC3339)
		case string:
			candidate = strings.TrimSpace(value)
		}
		if _, ok := parseLooseDataTime(candidate); ok {
			return candidate
		}
	}
	if info != nil {
		return info.ModTime().UTC().Format(time.RFC3339)
	}
	return "unknown"
}

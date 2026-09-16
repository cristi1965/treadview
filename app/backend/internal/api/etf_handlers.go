package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/models"
)

const etfAnalysesMaxAge = 7 * 24 * time.Hour

type etfAnalysesFile struct {
	Updated      string                   `json:"updated"`
	AUMUnit      string                   `json:"aumUnit"`
	N            int                      `json:"n"`
	Supers       map[string]int           `json:"supers"`
	Sectors      []etfSectorAnalysisEntry `json:"sectors"`
	ETFs         []etfAnalysisEntry       `json:"etfs"`
	Source       string                   `json:"source,omitempty"`
	SourceURL    string                   `json:"sourceURL,omitempty"`
	MetadataAsOf string                   `json:"metadataAsOf,omitempty"`
	Scope        etfAnalysisScope         `json:"scope,omitempty"`
}

type etfAnalysisScope struct {
	Kind      string `json:"kind"`
	Requested int    `json:"requested"`
	Completed int    `json:"completed"`
	Complete  bool   `json:"complete"`
}

type etfRefreshDiagnostics struct {
	AgeSeconds        int64  `json:"ageSeconds"`
	MaxAgeSeconds     int64  `json:"maxAgeSeconds"`
	LastAttemptAt     string `json:"lastAttemptAt,omitempty"`
	NextScheduledAt   string `json:"nextScheduledAt,omitempty"`
	LastAttemptStatus string `json:"lastAttemptStatus,omitempty"`
	LastAttemptError  string `json:"lastAttemptError,omitempty"`
	ScopeRequested    int    `json:"scopeRequested,omitempty"`
	ScopeCompleted    int    `json:"scopeCompleted,omitempty"`
}

type etfSectorAnalysisEntry struct {
	Sector string  `json:"sector"`
	Super  string  `json:"super"`
	N      int     `json:"n"`
	AUM    float64 `json:"aum"`
}

type etfAnalysisEntry struct {
	Sym     string   `json:"sym"`
	Name    string   `json:"name"`
	AUM     float64  `json:"aum,omitempty"`
	Expense *float64 `json:"expense,omitempty"`
	Ret1Y   *float64 `json:"ret1y,omitempty"`
	Ret5Y   *float64 `json:"ret5y,omitempty"`
	MDD     *float64 `json:"mdd,omitempty"`
	Sector  string   `json:"sector"`
	Kind    string   `json:"kind"`
	Verdict string   `json:"verdict,omitempty"`
}

func (h *Handler) TriggerETFRefresh(c *gin.Context) {
	if h == nil || h.etfRefresh == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ETF refresh service unavailable"})
		return
	}
	queued := h.etfRefresh.Trigger()
	status := http.StatusAccepted
	message := "ETF refresh queued"
	if !queued {
		status = http.StatusConflict
		message = "ETF refresh already queued or running"
	}
	c.JSON(status, gin.H{"status": map[bool]string{true: "queued", false: "busy"}[queued], "message": message})
}

// GetETFSectors handles GET /api/etf/sectors
func (h *Handler) GetETFSectors(c *gin.Context) {
	category := c.Query("category")         // Filter by category
	sortBy := c.DefaultQuery("sort", "aum") // Sort by: aum, return1y, return5y, drawdown
	search := c.Query("q")                  // Search query

	mode, ok := bindETFReadMode(c)
	if !ok {
		return
	}
	data, src, degradedReason, err := loadETFAnalysesForMode(mode)
	diagnostics := buildETFRefreshDiagnostics(data)
	if err != nil {
		setDataFreshness(c, dataFreshnessMeta{Source: src, DataTime: data.Updated, Stale: true, StaleReason: err.Error(), Refreshable: true})
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "dataMode": mode, "updated": data.Updated, "diagnostics": diagnostics})
		return
	}
	if mode == "historical" {
		setDataFreshness(c, dataFreshnessMeta{
			Source:      src,
			DataTime:    data.Updated,
			Stale:       true,
			StaleReason: "historical ETF snapshot; not for current decisions",
			Refreshable: true,
		})
	}
	if c.Writer.Header().Get("X-Data-Source") == "" {
		setDataFreshness(c, dataFreshnessMeta{Source: src, DataTime: data.Updated, Refreshable: true})
	}
	sectors := etfSectorsFromAnalysis(data)

	// Apply category filter
	if category != "" && category != "all" {
		filtered := []models.ETFSector{}
		for _, s := range sectors {
			if string(s.Category) == category {
				filtered = append(filtered, s)
			}
		}
		sectors = filtered
	}

	// Apply search filter
	if search != "" {
		filtered := []models.ETFSector{}
		searchLower := strings.ToLower(search)
		for _, s := range sectors {
			if strings.Contains(strings.ToLower(s.Name), searchLower) ||
				strings.Contains(strings.ToLower(s.TopPerformer.Ticker), searchLower) {
				filtered = append(filtered, s)
			}
		}
		sectors = filtered
	}

	// Apply sorting
	sectors = sortETFSectors(sectors, sortBy)

	// Count by category
	categoryCounts := make(map[models.ETFCategory]int)
	allSectors := etfSectorsFromAnalysis(data)
	for _, s := range allSectors {
		categoryCounts[s.Category]++
	}

	c.JSON(http.StatusOK, gin.H{
		"sectors":        sectors,
		"total":          len(sectors),
		"categories":     categoryCounts,
		"etfs":           data.ETFs,
		"aumUnit":        data.AUMUnit,
		"updated":        data.Updated,
		"priceAsOf":      data.Updated,
		"metadataAsOf":   data.MetadataAsOf,
		"dataMode":       mode,
		"degradedReason": degradedReason,
		"diagnostics":    diagnostics,
	})
}

// GetETFSectorDetail handles GET /api/etf/sectors/:id
func (h *Handler) GetETFSectorDetail(c *gin.Context) {
	sectorID := c.Param("id")

	mode, ok := bindETFReadMode(c)
	if !ok {
		return
	}
	data, src, degradedReason, err := loadETFAnalysesForMode(mode)
	if err != nil {
		writeETFUnavailable(c, mode, data, src, err)
		return
	}
	setETFReadFreshness(c, src, data.Updated, mode, degradedReason)
	sectors := etfSectorsFromAnalysis(data)

	var sector *models.ETFSector
	for _, s := range sectors {
		if s.ID == sectorID {
			sector = &s
			break
		}
	}

	if sector == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sector not found"})
		return
	}

	etfs := make([]etfAnalysisEntry, 0)
	for _, etf := range data.ETFs {
		if etf.Sector == sectorID || slugifyETFSector(etf.Sector) == sectorID {
			etfs = append(etfs, etf)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sector": sector, "etfs": etfs, "updated": data.Updated,
		"dataMode": mode, "degradedReason": degradedReason, "diagnostics": buildETFRefreshDiagnostics(data),
	})
}

// SearchETFs handles GET /api/etf/search
func (h *Handler) SearchETFs(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	mode, ok := bindETFReadMode(c)
	if !ok {
		return
	}
	data, src, degradedReason, err := loadETFAnalysesForMode(mode)
	if err != nil {
		writeETFUnavailable(c, mode, data, src, err)
		return
	}
	setETFReadFreshness(c, src, data.Updated, mode, degradedReason)

	q := strings.ToLower(strings.TrimSpace(query))
	results := make([]models.ETF, 0, 20)
	for _, etf := range data.ETFs {
		if !strings.Contains(strings.ToLower(etf.Sym), q) && !strings.Contains(strings.ToLower(etf.Name), q) {
			continue
		}
		results = append(results, etfToModel(etf))
		if len(results) >= 20 {
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results, "count": len(results), "updated": data.Updated,
		"dataMode": mode, "degradedReason": degradedReason, "diagnostics": buildETFRefreshDiagnostics(data),
	})
}

func writeETFUnavailable(c *gin.Context, mode string, data etfAnalysesFile, source string, err error) {
	setDataFreshness(c, dataFreshnessMeta{
		Source: source, DataTime: data.Updated, Stale: true, StaleReason: err.Error(), Refreshable: true,
	})
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": err.Error(), "dataMode": mode, "updated": data.Updated, "diagnostics": buildETFRefreshDiagnostics(data),
	})
}

func bindETFReadMode(c *gin.Context) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(c.Query("mode")))
	switch value {
	case "", "current":
		return "current", true
	case "historical":
		return "historical", true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported ETF data mode %q; use current or historical", value)})
		return "", false
	}
}

func loadETFAnalysesForMode(mode string) (etfAnalysesFile, string, string, error) {
	if mode == "historical" {
		data, src, err := loadHistoricalETFAnalysesFile()
		return data, src, "historical ETF snapshot; not for current decisions", err
	}
	data, src, err := loadFreshETFAnalyses()
	return data, src, "", err
}

func setETFReadFreshness(c *gin.Context, source, dataTime, mode, degradedReason string) {
	meta := dataFreshnessMeta{Source: source, DataTime: dataTime, Refreshable: true}
	if mode == "historical" || degradedReason != "" {
		meta.Stale = true
		meta.StaleReason = "historical ETF snapshot; not for current decisions"
	}
	setDataFreshness(c, meta)
}

func loadFreshETFAnalyses() (etfAnalysesFile, string, error) {
	data, src, err := loadCurrentETFAnalysesFile()
	if err != nil {
		return etfAnalysesFile{}, src, err
	}
	if err := validateCurrentETFAnalysis(data); err != nil {
		return data, src, err
	}
	updatedAt, err := parseETFUpdated(data.Updated)
	if err != nil {
		return data, src, fmt.Errorf("ETF analyses updated field invalid: %s", data.Updated)
	}
	age := time.Since(updatedAt)
	if age < 0 {
		return data, src, fmt.Errorf("ETF analyses updated field is in the future: %s", data.Updated)
	}
	if age > etfAnalysesMaxAge {
		return data, src, fmt.Errorf("ETF analyses stale: updated=%s age=%.1fh max=%.0fh", data.Updated, age.Hours(), etfAnalysesMaxAge.Hours())
	}
	return data, src, nil
}

func validateCurrentETFAnalysis(data etfAnalysesFile) error {
	if len(data.ETFs) == 0 || data.N != len(data.ETFs) {
		return fmt.Errorf("ETF current scope count invalid: n=%d etfs=%d", data.N, len(data.ETFs))
	}
	if !data.Scope.Complete || data.Scope.Requested != len(data.ETFs) || data.Scope.Completed != len(data.ETFs) {
		return fmt.Errorf("ETF current scope incomplete: requested=%d completed=%d rows=%d", data.Scope.Requested, data.Scope.Completed, len(data.ETFs))
	}
	if strings.TrimSpace(data.Source) == "" || strings.TrimSpace(data.MetadataAsOf) == "" {
		return fmt.Errorf("ETF current provenance or metadata date missing")
	}
	metadataAt, err := parseETFUpdated(data.MetadataAsOf)
	if err != nil || metadataAt.After(time.Now()) {
		return fmt.Errorf("ETF metadataAsOf invalid: %s", data.MetadataAsOf)
	}
	for _, item := range data.ETFs {
		if strings.TrimSpace(item.Sym) == "" || strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Sector) == "" || strings.TrimSpace(item.Kind) == "" || item.AUM <= 0 || item.Expense == nil || *item.Expense < 0 || item.Ret1Y == nil || item.Ret5Y == nil || item.MDD == nil {
			return fmt.Errorf("ETF current metrics incomplete for %s", item.Sym)
		}
	}
	return nil
}

func loadCurrentETFAnalysesFile() (etfAnalysesFile, string, error) {
	return loadETFAnalysesFromPaths([]string{
		filepath.Join("data", "etf-analyses-current.json"),
		filepath.Join("app", "backend", "data", "etf-analyses-current.json"),
	})
}

func loadHistoricalETFAnalysesFile() (etfAnalysesFile, string, error) {
	paths := latestETFHistoryPaths([]string{
		filepath.Join("data", "etf-history"),
		filepath.Join("app", "backend", "data", "etf-history"),
	})
	paths = append(paths,
		filepath.Join("data", "etf-analyses.json"),
		filepath.Join("app", "backend", "data", "etf-analyses.json"),
		filepath.Join("..", "frontend", "public", "data", "etf-analyses.json"),
		filepath.Join("app", "frontend", "public", "data", "etf-analyses.json"),
		filepath.Join("..", "frontend", "dist", "data", "etf-analyses.json"),
		filepath.Join("app", "frontend", "dist", "data", "etf-analyses.json"),
	)
	return loadETFAnalysesFromPaths(paths)
}

func latestETFHistoryPaths(dirs []string) []string {
	paths := make([]string, 0)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
				continue
			}
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return filepath.Base(paths[i]) > filepath.Base(paths[j])
	})
	return paths
}

func loadETFAnalysesFromPaths(paths []string) (etfAnalysesFile, string, error) {
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var data etfAnalysesFile
		if err := json.Unmarshal(raw, &data); err != nil {
			return etfAnalysesFile{}, path, err
		}
		if len(data.Sectors) == 0 {
			return etfAnalysesFile{}, path, fmt.Errorf("ETF analyses empty: %s", path)
		}
		if err := normalizeETFAnalysisAUM(&data); err != nil {
			return etfAnalysesFile{}, path, fmt.Errorf("ETF analyses AUM contract invalid: %w", err)
		}
		return data, "etf-analyses@" + path, nil
	}
	return etfAnalysesFile{}, "missing", fmt.Errorf("ETF analyses file not found")
}

func buildETFRefreshDiagnostics(data etfAnalysesFile) etfRefreshDiagnostics {
	diagnostics := etfRefreshDiagnostics{AgeSeconds: -1, MaxAgeSeconds: int64(etfAnalysesMaxAge.Seconds())}
	if updatedAt, err := parseETFUpdated(data.Updated); err == nil {
		diagnostics.AgeSeconds = int64(time.Since(updatedAt).Seconds())
	}
	for _, path := range []string{
		filepath.Join("data", "etf-refresh-status.json"),
		filepath.Join("app", "backend", "data", "etf-refresh-status.json"),
	} {
		raw, err := os.ReadFile(path)
		if err == nil && json.Unmarshal(raw, &diagnostics) == nil {
			diagnostics.MaxAgeSeconds = int64(etfAnalysesMaxAge.Seconds())
			if updatedAt, parseErr := parseETFUpdated(data.Updated); parseErr == nil {
				diagnostics.AgeSeconds = int64(time.Since(updatedAt).Seconds())
			}
			break
		}
	}
	return diagnostics
}

func normalizeETFAnalysisAUM(data *etfAnalysesFile) error {
	if data == nil {
		return fmt.Errorf("missing dataset")
	}
	switch strings.ToLower(strings.TrimSpace(data.AUMUnit)) {
	case "usd":
		return nil
	case "usd_thousands":
		for index := range data.Sectors {
			data.Sectors[index].AUM *= 1000
		}
		for index := range data.ETFs {
			data.ETFs[index].AUM *= 1000
		}
		data.AUMUnit = "usd"
		return nil
	default:
		return fmt.Errorf("unsupported or missing aumUnit %q", data.AUMUnit)
	}
}

func parseETFUpdated(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty updated")
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func etfSectorsFromAnalysis(data etfAnalysesFile) []models.ETFSector {
	bySector := map[string][]etfAnalysisEntry{}
	for _, etf := range data.ETFs {
		bySector[etf.Sector] = append(bySector[etf.Sector], etf)
	}

	out := make([]models.ETFSector, 0, len(data.Sectors))
	for _, sec := range data.Sectors {
		members := bySector[sec.Sector]
		sector := models.ETFSector{
			ID:          slugifyETFSector(sec.Sector),
			Name:        sec.Sector,
			Category:    etfCategoryFromSuper(sec.Super),
			ETFCount:    sec.N,
			AUM:         sec.AUM,
			MaxDrawdown: minETFDrawdown(members),
			Return1Y:    floatPtr(avgETFReturn(members, "1y")),
			Return5Y:    floatPtr(avgETFReturn(members, "5y")),
		}
		top := topETFPerformer(members)
		sector.TopPerformer.Ticker = top.Sym
		if top.Ret5Y != nil {
			sector.TopPerformer.Return5Y = *top.Ret5Y
		}
		out = append(out, sector)
	}
	return out
}

func etfCategoryFromSuper(super string) models.ETFCategory {
	switch super {
	case "宽基":
		return models.CategoryBroad
	case "行业":
		return models.CategoryIndustry
	case "主题":
		return models.CategoryTheme
	case "因子策略":
		return models.CategoryFactor
	case "债券":
		return models.CategoryBond
	case "商品":
		return models.CategoryCommodity
	case "工具":
		return models.CategoryLeveraged
	default:
		return models.CategoryOther
	}
}

func slugifyETFSector(name string) string {
	slug := strings.ToLower(name)
	slug = strings.Map(func(r rune) rune {
		if r == ' ' || r == '/' || r == '_' || r == '、' {
			return '-'
		}
		return r
	}, slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "sector"
	}
	return slug
}

func avgETFReturn(members []etfAnalysisEntry, horizon string) float64 {
	sum := 0.0
	count := 0
	for _, member := range members {
		var value *float64
		if horizon == "1y" {
			value = member.Ret1Y
		} else {
			value = member.Ret5Y
		}
		if value == nil {
			continue
		}
		sum += *value
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func minETFDrawdown(members []etfAnalysisEntry) float64 {
	min := 0.0
	for _, member := range members {
		if member.MDD != nil && *member.MDD < min {
			min = *member.MDD
		}
	}
	return min
}

func topETFPerformer(members []etfAnalysisEntry) etfAnalysisEntry {
	if len(members) == 0 {
		return etfAnalysisEntry{Sym: "—"}
	}
	best := members[0]
	for _, candidate := range members[1:] {
		if candidate.Ret5Y != nil && (best.Ret5Y == nil || *candidate.Ret5Y > *best.Ret5Y) {
			best = candidate
		}
	}
	return best
}

func etfToModel(etf etfAnalysisEntry) models.ETF {
	return models.ETF{
		Ticker:       etf.Sym,
		Name:         etf.Name,
		Category:     etfCategoryFromSuper(etf.Kind),
		Sector:       etf.Sector,
		AUM:          etf.AUM,
		ExpenseRatio: derefFloat(etf.Expense),
		Return1Y:     etf.Ret1Y,
		Return5Y:     etf.Ret5Y,
		MaxDrawdown:  derefFloat(etf.MDD),
	}
}

func derefFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

// Helper function to sort ETF sectors
func sortETFSectors(sectors []models.ETFSector, sortBy string) []models.ETFSector {
	sorted := make([]models.ETFSector, len(sectors))
	copy(sorted, sectors)
	value := func(sector models.ETFSector) float64 {
		switch sortBy {
		case "return1y":
			if sector.Return1Y != nil {
				return *sector.Return1Y
			}
		case "return5y":
			if sector.Return5Y != nil {
				return *sector.Return5Y
			}
		case "drawdown":
			return sector.MaxDrawdown
		default:
			return sector.AUM
		}
		return 0
	}
	sort.SliceStable(sorted, func(i, j int) bool { return value(sorted[i]) > value(sorted[j]) })

	return sorted
}

func floatPtr(f float64) *float64 {
	return &f
}

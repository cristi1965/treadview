package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/dataflows"
	"trading-agents/internal/models"
)

const (
	flashDefaultLimit = 60
	flashMaxLimit     = 200
	// Return the last verified snapshot before the frontend's 5s request deadline.
	// Equal client/server deadlines race and turn a valid degraded response into a false failure.
	flashLiveTimeout = 4 * time.Second
	flashFreshFor    = 6 * time.Hour
)

// GetFlash 处理 GET /api/flash —— 金十风格的 7×24 快讯流。
//
// query 参数：
//
//	limit       返回条数，默认 60，取值范围 [1,200]，越界自动夹紧。
//	importance  重要性下限过滤：
//	              all（默认）    = 不过滤，1/2/3 全返回
//	              important      = 只返回 importance >= 2（关注 + 重磅）
//	              critical / top = 只返回 importance >= 3（只看重磅红）
//	              1 / 2 / 3      = 只返回 importance >= 该数字
//	kind        类型过滤，取 macro|earnings|company|market|calendar，
//	            可用逗号分隔多选；不传或传 all 表示不过滤。
//
// 过滤先于截断：先按 importance/kind 筛，再取前 limit 条。
//
// 数据来源：dataflows 聚合公开 RSS（90 秒内存缓存）。抓取失败或结果为空时
// 降级读静态种子文件 data/flash-live.json（或 app/backend/data/flash-live.json）。
// 设置 STOCKGOD_FLASH_OFFLINE=true 可跳过联网，直接用静态文件（本地/测试用）。
func GetFlash(c *gin.Context) {
	limit := clampFlashLimit(c.Query("limit"))
	minImportance := parseFlashImportance(c.Query("importance"))
	kinds := parseFlashKinds(c.Query("kind"))

	var (
		items    []models.FlashItem
		updated  string
		dataSrc  = "flash-live"
		liveFail bool
	)

	if !flashOfflineMode() {
		fetchDone := make(chan struct {
			items []models.FlashItem
			err   error
		}, 1)
		go func() {
			fetched, err := dataflows.DefaultFlashClient().GetFlash(0)
			fetchDone <- struct {
				items []models.FlashItem
				err   error
			}{fetched, err}
		}()
		var fetched []models.FlashItem
		var err error
		select {
		case result := <-fetchDone:
			fetched, err = result.items, result.err
		case <-time.After(flashLiveTimeout):
			liveFail = true
			log.Printf("[flash] live fetch timed out after %s", flashLiveTimeout)
		}
		if err != nil || len(fetched) == 0 {
			liveFail = true
		} else {
			items = fetched
			if observedAt, ok := latestFlashObservation(items); ok {
				updated = observedAt.Format(time.RFC3339)
			}
		}
	} else {
		liveFail = true
	}

	if liveFail {
		staticItems, staticUpdated, ok := loadFlashFromJSON()
		if !ok {
			staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
				Source:      "missing",
				Refreshable: true,
			}, "flash live source unavailable and no real snapshot exists")
			return
		}
		items = staticItems
		updated = staticUpdated
		if observedAt, observed := latestFlashObservation(items); observed {
			updated = observedAt.Format(time.RFC3339)
		}
		dataSrc = realSnapshotSource("flash-live")
	}

	items = filterFlashItems(items, minImportance, kinds)
	if len(items) > limit {
		items = items[:limit]
	}
	if observedAt, observed := latestFlashObservation(items); observed {
		updated = observedAt.Format(time.RFC3339)
	}
	if items == nil {
		items = []models.FlashItem{}
	}
	if updated == "" {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      dataSrc,
			Refreshable: true,
		}, "flash data has no trustworthy timestamp")
		return
	}

	stale := liveFail
	reason := ""
	if stale {
		reason = "flash live source unavailable; serving last real snapshot"
	}
	if old, ageReason := staleIfOlder(updated, flashFreshFor); old {
		stale = true
		if reason != "" {
			reason += "; "
		}
		reason += ageReason
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      dataSrc,
		DataTime:    updated,
		Stale:       stale,
		StaleReason: reason,
		Refreshable: true,
	})
	c.JSON(http.StatusOK, models.FlashResponse{Items: items, UpdatedAt: updated})
}

func latestFlashObservation(items []models.FlashItem) (time.Time, bool) {
	latest := time.Time{}
	for _, item := range items {
		observedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.Time))
		if err == nil && observedAt.After(latest) {
			latest = observedAt
		}
	}
	return latest.UTC(), !latest.IsZero()
}

func flashOfflineMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("STOCKGOD_FLASH_OFFLINE")), "true")
}

func clampFlashLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return flashDefaultLimit
	}
	if limit > flashMaxLimit {
		return flashMaxLimit
	}
	return limit
}

// parseFlashImportance 返回重要性下限（1 表示不过滤）。
func parseFlashImportance(raw string) int {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "all", "any", "1":
		return 1
	case "important", "2":
		return 2
	case "critical", "top", "red", "3":
		return 3
	default:
		return 1
	}
}

// parseFlashKinds 返回需要保留的 kind 集合；nil 表示不过滤。
func parseFlashKinds(raw string) map[string]bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" || raw == "all" {
		return nil
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if k := strings.TrimSpace(part); k != "" {
			out[k] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterFlashItems(items []models.FlashItem, minImportance int, kinds map[string]bool) []models.FlashItem {
	if minImportance <= 1 && kinds == nil {
		return items
	}
	out := make([]models.FlashItem, 0, len(items))
	for _, it := range items {
		if it.Importance < minImportance {
			continue
		}
		if kinds != nil && !kinds[strings.ToLower(it.Kind)] {
			continue
		}
		out = append(out, it)
	}
	return out
}

// flashFile 兼容两种静态格式：{items,updatedAt} 对象，或裸数组。
type flashFile struct {
	Items     []models.FlashItem `json:"items"`
	UpdatedAt string             `json:"updatedAt"`
}

// loadFlashFromJSON 双路径读取静态种子（与 loadReportsFromJSON 同款写法）。
func loadFlashFromJSON() ([]models.FlashItem, string, bool) {
	data, err := os.ReadFile(filepath.Join("data", "flash-live.json"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("app", "backend", "data", "flash-live.json"))
	}
	if err != nil || len(data) == 0 {
		return nil, "", false
	}

	var file flashFile
	if err := json.Unmarshal(data, &file); err == nil && len(file.Items) > 0 {
		return sortFlashDesc(file.Items), file.UpdatedAt, true
	}

	var bare []models.FlashItem
	if err := json.Unmarshal(data, &bare); err == nil && len(bare) > 0 {
		return sortFlashDesc(bare), "", true
	}
	return nil, "", false
}

func sortFlashDesc(items []models.FlashItem) []models.FlashItem {
	out := make([]models.FlashItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out
}

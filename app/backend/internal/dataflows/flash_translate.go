package dataflows

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/config"
	"trading-agents/internal/llm"
	"trading-agents/internal/models"
)

// 快讯标题中文化：批量调用 LLM 翻译英文标题，结果落盘缓存。
//
// 没有 Key / 请求失败 / 超时时一律降级为「直接用英文原标题」，
// 绝不再拼「关键词｜英文」这种半成品串。

const (
	// 整轮翻译（含所有批次）的硬超时，超了就用已拿到的部分，其余降级英文。
	flashTranslateTimeout = 10 * time.Second
	// 一次请求翻多少条。批太大容易被截断，太小又浪费请求。
	flashTranslateBatch = 25
	// 一轮最多翻多少条新标题，避免首次冷启动把 10s 预算打满。
	flashTranslateMaxPerRun = 60
	// 磁盘缓存条数上限，超出按最近使用时间淘汰最旧的。
	flashTitleCacheMax = 2000
)

const flashTranslateSystemPrompt = `你是财经快讯编辑，把英文财经新闻标题翻译成简体中文。
要求：
- 新闻快讯口吻，一句话，简洁通顺，不超过 40 字
- 不加引号、不加书名号、不加任何解释或前缀
- 保留股票代码（如 NVDA）、公司名、数字与百分比
- 只翻译，不评论、不补充原文没有的信息
只输出 JSON：{"items":[{"i":0,"zh":"中文标题"}]}，i 与输入编号一一对应。`

// FlashTitleTranslator 把英文标题批量翻成中文。
// 返回值与入参等长，某一项为空字符串表示该条未翻出（调用方降级英文原标题）。
type FlashTitleTranslator interface {
	TranslateTitles(ctx context.Context, titles []string) ([]string, error)
}

// --- 磁盘缓存 ---

type flashTitleCacheEntry struct {
	ZH string `json:"zh"`
	At int64  `json:"at"` // 最近命中/写入的 Unix 毫秒，用于淘汰
}

type flashTitleCacheFile struct {
	Updated string                          `json:"updated"`
	Titles  map[string]flashTitleCacheEntry `json:"titles"`
}

// flashTitleCache 是并发安全的标题翻译缓存（内存 + JSON 落盘）。
type flashTitleCache struct {
	mu      sync.Mutex
	path    string
	max     int
	entries map[string]flashTitleCacheEntry
	loaded  bool
	dirty   bool
}

func newFlashTitleCache(path string, max int) *flashTitleCache {
	if max <= 0 {
		max = flashTitleCacheMax
	}
	return &flashTitleCache{path: path, max: max, entries: map[string]flashTitleCacheEntry{}}
}

func flashTitleKey(titleEn string) string {
	sum := sha1.Sum([]byte(normalizeFlashTitle(titleEn)))
	return hex.EncodeToString(sum[:])
}

// load 惰性读盘，只做一次；文件不存在或损坏都当空缓存。
func (c *flashTitleCache) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	if c.path == "" {
		return
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	var file flashTitleCacheFile
	if err := json.Unmarshal(raw, &file); err != nil {
		log.Printf("[flash/translate] cache parse: %v", err)
		return
	}
	for k, v := range file.Titles {
		if strings.TrimSpace(v.ZH) != "" {
			c.entries[k] = v
		}
	}
}

// Get 返回缓存的中文标题，并刷新其最近使用时间。
func (c *flashTitleCache) Get(titleEn string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	e, ok := c.entries[flashTitleKey(titleEn)]
	if !ok {
		return "", false
	}
	if now := time.Now().UnixMilli(); now-e.At > int64(time.Hour/time.Millisecond) {
		e.At = now
		c.entries[flashTitleKey(titleEn)] = e
		c.dirty = true
	}
	return e.ZH, true
}

// Put 写入一条翻译（空值忽略）。
func (c *flashTitleCache) Put(titleEn, zh string) {
	zh = strings.TrimSpace(zh)
	if zh == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	c.entries[flashTitleKey(titleEn)] = flashTitleCacheEntry{ZH: zh, At: time.Now().UnixMilli()}
	c.dirty = true
}

// evictLocked 超过上限时按最近使用时间淘汰最旧的条目。
func (c *flashTitleCache) evictLocked() {
	if len(c.entries) <= c.max {
		return
	}
	type kv struct {
		key string
		at  int64
	}
	all := make([]kv, 0, len(c.entries))
	for k, v := range c.entries {
		all = append(all, kv{k, v.At})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].at != all[j].at {
			return all[i].at < all[j].at
		}
		return all[i].key < all[j].key
	})
	for _, item := range all[:len(all)-c.max] {
		delete(c.entries, item.key)
	}
}

// Flush 把缓存写盘（先写临时文件再 rename，避免半截文件）。
func (c *flashTitleCache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty || c.path == "" {
		return nil
	}
	c.evictLocked()

	snapshot := make(map[string]flashTitleCacheEntry, len(c.entries))
	for k, v := range c.entries {
		snapshot[k] = v
	}
	raw, err := json.MarshalIndent(flashTitleCacheFile{
		Updated: time.Now().UTC().Format(time.RFC3339),
		Titles:  snapshot,
	}, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(c.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}
	c.dirty = false
	return nil
}

// Len 返回当前条数（测试用）。
func (c *flashTitleCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	return len(c.entries)
}

// DefaultFlashTitleCachePath 兼容从仓库根或 app/backend 启动两种工作目录。
func DefaultFlashTitleCachePath() string {
	candidates := []string{
		filepath.Join("data", "flash-title-cache.json"),
		filepath.Join("app", "backend", "data", "flash-title-cache.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			return p
		}
	}
	return candidates[0]
}

// --- LLM 翻译器 ---

type llmFlashTranslator struct{ client llm.LLMClient }

type flashTranslateResponse struct {
	Items []struct {
		I  int    `json:"i"`
		ZH string `json:"zh"`
	} `json:"items"`
}

func (t *llmFlashTranslator) TranslateTitles(ctx context.Context, titles []string) ([]string, error) {
	out := make([]string, len(titles))
	if t == nil || t.client == nil || len(titles) == 0 {
		return out, fmt.Errorf("flash translator unavailable")
	}

	var b strings.Builder
	b.WriteString("把下面每条英文财经标题翻译成中文快讯：\n")
	for i, title := range titles {
		fmt.Fprintf(&b, "%d. %s\n", i, title)
	}
	b.WriteString("\n只输出 JSON：{\"items\":[{\"i\":0,\"zh\":\"...\"}]}")

	var resp flashTranslateResponse
	// quick 模型足够，翻译不需要深思考。
	if err := t.client.StructuredGenerate(ctx, flashTranslateSystemPrompt, b.String(), false, &resp); err != nil {
		return out, err
	}
	for _, item := range resp.Items {
		if item.I < 0 || item.I >= len(out) {
			continue
		}
		out[item.I] = sanitizeFlashZHTitle(item.ZH)
	}
	return out, nil
}

// sanitizeFlashZHTitle 去掉模型偶尔加的引号 / 序号 / 前缀。
func sanitizeFlashZHTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'“”「」《》 ")
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 80 {
		s = string([]rune(s)[:80])
	}
	return s
}

var (
	defaultFlashTranslatorOnce sync.Once
	defaultFlashTranslator     FlashTitleTranslator
)

// DefaultFlashTitleTranslator 按 .env 里的 provider 建 LLM 客户端；
// 没配 Key 或建不出来就返回 nil（调用方降级为英文原标题）。
func DefaultFlashTitleTranslator() FlashTitleTranslator {
	defaultFlashTranslatorOnce.Do(func() {
		cfg := config.Load()
		if !flashLLMKeyAvailable(cfg) {
			log.Println("[flash/translate] no LLM API key — falling back to English headlines")
			return
		}
		client, err := llm.NewClient(cfg)
		if err != nil {
			log.Printf("[flash/translate] llm client: %v", err)
			return
		}
		defaultFlashTranslator = &llmFlashTranslator{client: client}
	})
	return defaultFlashTranslator
}

func flashLLMKeyAvailable(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "gemini", "google", "":
		return cfg.GoogleAPIKey != ""
	case "deepseek":
		return cfg.DeepSeekAPIKey != ""
	case "openai":
		return cfg.OpenAIAPIKey != ""
	case "openai_compatible":
		return cfg.LLMBackendURL != ""
	default:
		return false
	}
}

// --- 聚合阶段的翻译入口 ---

// TranslateFlashTitles 就地把 items 的 Title 换成中文。
//
// 流程：先查磁盘缓存，未命中的按批送 LLM（整轮 10s 预算），
// 拿到的写回缓存并落盘；仍未翻出的保持英文原标题。
// translator 为 nil 时整体降级，只走缓存。
func TranslateFlashTitles(ctx context.Context, items []models.FlashItem, translator FlashTitleTranslator, cache *flashTitleCache) {
	translateFlashTitles(ctx, items, translator, cache, true)
}

// TranslateFlashTitlesReadOnly applies cached/new translations without flushing request data to disk.
func TranslateFlashTitlesReadOnly(ctx context.Context, items []models.FlashItem, translator FlashTitleTranslator, cache *flashTitleCache) {
	translateFlashTitles(ctx, items, translator, cache, false)
}

func translateFlashTitles(ctx context.Context, items []models.FlashItem, translator FlashTitleTranslator, cache *flashTitleCache, persist bool) {
	if len(items) == 0 {
		return
	}

	pendingIdx := make([]int, 0, len(items))
	for i := range items {
		titleEn := strings.TrimSpace(items[i].TitleEn)
		if titleEn == "" {
			continue
		}
		if cache != nil {
			if zh, ok := cache.Get(titleEn); ok {
				items[i].Title = zh
				continue
			}
		}
		items[i].Title = titleEn // 先降级，翻出来再覆盖
		pendingIdx = append(pendingIdx, i)
	}

	if translator == nil || len(pendingIdx) == 0 {
		return
	}
	if len(pendingIdx) > flashTranslateMaxPerRun {
		pendingIdx = pendingIdx[:flashTranslateMaxPerRun]
	}

	ctx, cancel := context.WithTimeout(ctx, flashTranslateTimeout)
	defer cancel()

	translated := 0
	for start := 0; start < len(pendingIdx); start += flashTranslateBatch {
		if ctx.Err() != nil {
			break
		}
		end := start + flashTranslateBatch
		if end > len(pendingIdx) {
			end = len(pendingIdx)
		}
		batch := pendingIdx[start:end]

		titles := make([]string, len(batch))
		for k, idx := range batch {
			titles[k] = items[idx].TitleEn
		}
		zhList, err := translator.TranslateTitles(ctx, titles)
		if err != nil {
			log.Printf("[flash/translate] batch failed (%d titles): %v", len(titles), err)
		}
		for k, idx := range batch {
			if k >= len(zhList) {
				break
			}
			zh := sanitizeFlashZHTitle(zhList[k])
			if zh == "" {
				continue
			}
			items[idx].Title = zh
			translated++
			if cache != nil {
				cache.Put(items[idx].TitleEn, zh)
			}
		}
	}

	if persist && cache != nil && translated > 0 {
		if err := cache.Flush(); err != nil {
			log.Printf("[flash/translate] cache flush: %v", err)
		}
	}
	log.Printf("[flash/translate] translated=%d pending=%d", translated, len(pendingIdx))
}

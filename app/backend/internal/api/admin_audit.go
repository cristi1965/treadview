package api

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

const (
	requestIDHeader                     = "X-Request-ID"
	auditOutcomeHeader                  = "X-Audit-Outcome"
	auditErrorHeader                    = "X-Audit-Error"
	legacyAuditHashVersion              = 2
	auditHashVersion                    = 4
	systemAuditHashVersion              = 3
	paperDailyBaselineAuditAction       = "paper-risk.daily-equity-baseline.create"
	paperDailyBaselineAuditTargetPrefix = "paper-daily-equity-baseline:"
)

var (
	auditWriteMu   sync.Mutex
	validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,80}$`)
)

type auditHashPayload struct {
	CreatedAt  string `json:"createdAt"`
	RequestID  string `json:"requestId"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Result     string `json:"result"`
	HTTPStatus int    `json:"httpStatus"`
	PrevHash   string `json:"prevHash"`
}

type auditHashPayloadV2 struct {
	CreatedAt   string `json:"createdAt"`
	RequestID   string `json:"requestId"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Target      string `json:"target"`
	HashVersion int    `json:"hashVersion"`
	PrevHash    string `json:"prevHash"`
}

type auditHashPayloadV4 struct {
	CreatedAt   string `json:"createdAt"`
	RequestID   string `json:"requestId"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Target      string `json:"target"`
	PayloadHash string `json:"payloadHash"`
	HashVersion int    `json:"hashVersion"`
	PrevHash    string `json:"prevHash"`
}

var errDuplicateAuditRequest = errors.New("duplicate audit request")

type auditHashPayloadV3 struct {
	CreatedAt   string `json:"createdAt"`
	RequestID   string `json:"requestId"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Target      string `json:"target"`
	PayloadHash string `json:"payloadHash"`
	Status      string `json:"status"`
	HTTPStatus  int    `json:"httpStatus"`
	OutcomeHash string `json:"outcomeHash"`
	CompletedAt string `json:"completedAt"`
	HashVersion int    `json:"hashVersion"`
	PrevHash    string `json:"prevHash"`
}

type auditOutcomeHashPayload struct {
	AuditHash   string `json:"auditHash"`
	Status      string `json:"status"`
	HTTPStatus  int    `json:"httpStatus"`
	CompletedAt string `json:"completedAt"`
}

type adminAuthDecision struct {
	actor      string
	status     string
	httpStatus int
	message    string
	allowed    bool
}

type bufferedAuditResponseWriter struct {
	underlying gin.ResponseWriter
	header     http.Header
	body       bytes.Buffer
	status     int
}

func adminWriteGuard(cfg *config.Config, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestedID := normalizedRequestID(c.GetHeader(requestIDHeader))
		target := auditTarget(c)
		if database.DB == nil {
			c.Header(requestIDHeader, requestedID)
			c.Header(auditOutcomeHeader, "UNAVAILABLE")
			c.Header(auditErrorHeader, "pending-create-failed")
			log.Printf("audit pending create failed requestID=%s action=%s target=%s: audit store unavailable", requestedID, action, target)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "audit store unavailable", "requestId": requestedID})
			return
		}

		payloadHash, err := auditRequestPayloadHash(c.Request)
		if err != nil {
			c.Header(requestIDHeader, requestedID)
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large to audit", "requestId": requestedID})
			return
		}
		decision := decideAdminRequestAuthorization(cfg, c)
		pending, err := createPendingAuditOutcomeWithPayload(requestedID, decision.actor, action, target, payloadHash)
		if err != nil {
			if errors.Is(err, errDuplicateAuditRequest) {
				c.Header(requestIDHeader, requestedID)
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "request ID already used", "requestId": requestedID})
				return
			}
			c.Header(requestIDHeader, requestedID)
			c.Header(auditOutcomeHeader, "UNAVAILABLE")
			c.Header(auditErrorHeader, "pending-create-failed")
			log.Printf("audit pending create failed requestID=%s action=%s target=%s: %v", requestedID, action, target, err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "audit write failed; request denied", "requestId": requestedID})
			return
		}
		c.Header(requestIDHeader, pending.RequestID)

		if !decision.allowed {
			if err := finalizeAuditOutcome(pending.ID, decision.status, decision.httpStatus); err != nil {
				auditFinalizeFailure(c, pending, err)
				return
			}
			c.Header(auditOutcomeHeader, decision.status)
			c.AbortWithStatusJSON(decision.httpStatus, gin.H{"error": decision.message, "requestId": pending.RequestID})
			return
		}

		originalWriter := c.Writer
		bufferedWriter := newBufferedAuditResponseWriter(originalWriter)
		c.Writer = bufferedWriter
		c.Next()

		outcome := outcomeForHTTPStatus(bufferedWriter.Status())
		if err := finalizeAuditOutcome(pending.ID, outcome, bufferedWriter.Status()); err != nil {
			c.Writer = originalWriter
			auditFinalizeFailure(c, pending, err)
			return
		}
		bufferedWriter.Header().Set(auditOutcomeHeader, outcome)
		bufferedWriter.commit()
	}
}

func auditRequestPayloadHash(request *http.Request) (string, error) {
	const maxAuditedBody = 1 << 20
	raw := []byte{}
	if request.Body != nil {
		limited := io.LimitReader(request.Body, maxAuditedBody+1)
		var err error
		raw, err = io.ReadAll(limited)
		if err != nil {
			return "", err
		}
		if len(raw) > maxAuditedBody {
			return "", fmt.Errorf("request body exceeds %d bytes", maxAuditedBody)
		}
		request.Body = io.NopCloser(bytes.NewReader(raw))
	}
	bodySum := sha256.Sum256(raw)
	payload := struct {
		Query    string `json:"query"`
		BodyHash string `json:"bodyHash"`
	}{Query: request.URL.Query().Encode(), BodyHash: hex.EncodeToString(bodySum[:])}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func decideAdminAuthorization(cfg *config.Config, authorization string) adminAuthDecision {
	token := ""
	if cfg != nil {
		token = strings.TrimSpace(cfg.AdminToken)
	}
	if token == "" {
		return adminAuthDecision{actor: "anonymous", status: "DENIED_NOT_CONFIGURED", httpStatus: http.StatusServiceUnavailable, message: "admin authentication is not configured"}
	}
	authorization = strings.TrimSpace(authorization)
	if !strings.HasPrefix(authorization, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")) == "" {
		return adminAuthDecision{actor: "anonymous", status: "DENIED_MISSING_TOKEN", httpStatus: http.StatusUnauthorized, message: "Bearer token required"}
	}
	provided := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	expectedHash := sha256.Sum256([]byte(token))
	providedHash := sha256.Sum256([]byte(provided))
	if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
		return adminAuthDecision{actor: "invalid-bearer", status: "DENIED_INVALID_TOKEN", httpStatus: http.StatusForbidden, message: "invalid admin token"}
	}
	return adminAuthDecision{actor: "admin", allowed: true}
}

func decideAdminRequestAuthorization(cfg *config.Config, c *gin.Context) adminAuthDecision {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" {
		return decideAdminAuthorization(cfg, c.GetHeader("Authorization"))
	}
	if cfg == nil || strings.TrimSpace(cfg.AdminToken) == "" {
		return decideAdminAuthorization(cfg, "")
	}
	if validLocalAdminSession(c) {
		return adminAuthDecision{actor: "local-session", allowed: true}
	}
	return decideAdminAuthorization(cfg, "")
}

// adminReadGuard protects sensitive reads without mutating the audit chain being inspected.
func adminReadGuard(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := normalizedRequestID(c.GetHeader(requestIDHeader))
		c.Header(requestIDHeader, requestID)
		decision := decideAdminRequestAuthorization(cfg, c)
		if !decision.allowed {
			c.AbortWithStatusJSON(decision.httpStatus, gin.H{"error": decision.message, "requestId": requestID})
			return
		}
		c.Next()
	}
}

func adminMutatingMethodGuard(cfg *config.Config, action string) gin.HandlerFunc {
	guard := adminWriteGuard(cfg, action)
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
		default:
			guard(c)
		}
	}
}

func newBufferedAuditResponseWriter(underlying gin.ResponseWriter) *bufferedAuditResponseWriter {
	header := underlying.Header().Clone()
	return &bufferedAuditResponseWriter{underlying: underlying, header: header, status: http.StatusOK}
}

func (w *bufferedAuditResponseWriter) Header() http.Header { return w.header }

func (w *bufferedAuditResponseWriter) WriteHeader(status int) {
	if !w.Written() && status > 0 {
		w.status = status
	}
}

func (w *bufferedAuditResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(data)
}

func (w *bufferedAuditResponseWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(data)
}

func (w *bufferedAuditResponseWriter) Status() int { return w.status }
func (w *bufferedAuditResponseWriter) Size() int   { return w.body.Len() }
func (w *bufferedAuditResponseWriter) Written() bool {
	return w.body.Len() > 0 || w.status != http.StatusOK
}
func (w *bufferedAuditResponseWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}
func (w *bufferedAuditResponseWriter) Flush()                   {}
func (w *bufferedAuditResponseWriter) CloseNotify() <-chan bool { return w.underlying.CloseNotify() }
func (w *bufferedAuditResponseWriter) Pusher() http.Pusher      { return w.underlying.Pusher() }
func (w *bufferedAuditResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("audited response buffering does not support hijacking")
}

func (w *bufferedAuditResponseWriter) commit() {
	destination := w.underlying.Header()
	for key := range destination {
		destination.Del(key)
	}
	for key, values := range w.header {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
	w.underlying.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = w.underlying.Write(w.body.Bytes())
	}
}

func auditFinalizeFailure(c *gin.Context, pending models.AuditEvent, err error) {
	log.Printf("audit outcome remains PENDING requestID=%s action=%s target=%s: %v", pending.RequestID, pending.Action, pending.Target, err)
	c.Header(requestIDHeader, pending.RequestID)
	c.Header(auditOutcomeHeader, "PENDING")
	c.Header(auditErrorHeader, "finalize-failed")
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"error":       "business response withheld because audit outcome is still pending",
		"requestId":   pending.RequestID,
		"auditStatus": "PENDING",
	})
}

func outcomeForHTTPStatus(status int) string {
	switch {
	case status < http.StatusBadRequest:
		return "SUCCEEDED"
	case status < http.StatusInternalServerError:
		return "REJECTED"
	default:
		return "FAILED"
	}
}

func normalizedRequestID(value string) string {
	value = strings.TrimSpace(value)
	if validRequestID.MatchString(value) {
		return value
	}
	return uuid.NewString()
}

func auditTarget(c *gin.Context) string {
	target := c.FullPath()
	if target == "" {
		target = c.Request.URL.Path
	}
	if clientOrderID := strings.TrimSpace(c.Param("clientOrderId")); clientOrderID != "" {
		target += ":" + clientOrderID
	}
	if len(target) > 240 {
		target = target[:240]
	}
	return target
}

func createPendingAuditOutcome(requestID, actor, action, target string) (models.AuditEvent, error) {
	emptyHash := sha256.Sum256(nil)
	return createPendingAuditOutcomeWithPayload(requestID, actor, action, target, hex.EncodeToString(emptyHash[:]))
}

func createPendingAuditOutcomeWithPayload(requestID, actor, action, target, payloadHash string) (models.AuditEvent, error) {
	if database.DB == nil {
		return models.AuditEvent{}, errors.New("audit store unavailable")
	}
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()

	var event models.AuditEvent
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.AuditEvent{}).Where("request_id = ?", requestID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errDuplicateAuditRequest
		}
		event = models.AuditEvent{
			CreatedAt: time.Now().UTC(), RequestID: requestID, Actor: actor, Action: action, Target: target,
			Status: "PENDING", Result: "PENDING", PayloadHash: payloadHash, HashVersion: auditHashVersion,
		}
		var previous models.AuditEvent
		err := tx.Order("id desc").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			event.PrevHash = previous.Hash
		}
		event.Hash = calculateAuditHash(event)
		return tx.Create(&event).Error
	})
	return event, err
}

func finalizeAuditOutcome(id uint, status string, httpStatus int) error {
	if database.DB == nil {
		return errors.New("audit store unavailable")
	}
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var event models.AuditEvent
		if err := tx.First(&event, id).Error; err != nil {
			return err
		}
		if event.HashVersion < legacyAuditHashVersion || event.HashVersion == systemAuditHashVersion || event.Status != "PENDING" || event.CompletedAt != nil {
			return fmt.Errorf("audit outcome %d is not pending", id)
		}
		completedAt := time.Now().UTC()
		outcomeHash := calculateAuditOutcomeHash(event.Hash, status, httpStatus, completedAt)
		result := tx.Model(&models.AuditEvent{}).
			Where("id = ? AND status = ? AND completed_at IS NULL", id, "PENDING").
			Updates(map[string]any{
				"status": status, "result": status, "http_status": httpStatus,
				"completed_at": completedAt, "outcome_hash": outcomeHash,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("audit outcome %d was not finalized", id)
		}
		return nil
	})
}

// ReconcilePendingAuditOutcomes closes requests left in-flight by a previous
// process. It runs before serving new requests, so no request from the current
// process can legitimately still be pending.
func ReconcilePendingAuditOutcomes() (int, error) {
	if database.DB == nil {
		return 0, errors.New("audit store unavailable")
	}
	var pending []models.AuditEvent
	if err := database.DB.
		Where("hash_version IN ? AND status = ? AND completed_at IS NULL", []int{legacyAuditHashVersion, auditHashVersion}, "PENDING").
		Order("id asc").Find(&pending).Error; err != nil {
		return 0, err
	}
	resolved := 0
	for _, event := range pending {
		if err := finalizeAuditOutcome(event.ID, "FAILED", http.StatusServiceUnavailable); err != nil {
			return resolved, fmt.Errorf("reconcile pending audit outcome %d: %w", event.ID, err)
		}
		resolved++
	}
	return resolved, nil
}

// appendAuditEvent preserves the legacy append-only format for old callers and rows.
func appendAuditEvent(event models.AuditEvent) error {
	if database.DB == nil {
		return errors.New("audit store unavailable")
	}
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var previous models.AuditEvent
		err := tx.Order("id desc").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		event.CreatedAt = time.Now().UTC()
		event.HashVersion = 1
		if err == nil {
			event.PrevHash = previous.Hash
		}
		event.Hash = calculateAuditHash(event)
		return tx.Create(&event).Error
	})
}

func calculateAuditHash(event models.AuditEvent) string {
	if event.HashVersion == systemAuditHashVersion {
		completedAt := ""
		if event.CompletedAt != nil {
			completedAt = event.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		payload := auditHashPayloadV3{
			CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339Nano), RequestID: event.RequestID,
			Actor: event.Actor, Action: event.Action, Target: event.Target, PayloadHash: event.PayloadHash,
			Status: event.Status, HTTPStatus: event.HTTPStatus, OutcomeHash: event.OutcomeHash,
			CompletedAt: completedAt, HashVersion: event.HashVersion, PrevHash: event.PrevHash,
		}
		raw, _ := json.Marshal(payload)
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	if event.HashVersion >= auditHashVersion {
		payload := auditHashPayloadV4{
			CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339Nano), RequestID: event.RequestID,
			Actor: event.Actor, Action: event.Action, Target: event.Target, PayloadHash: event.PayloadHash,
			HashVersion: event.HashVersion, PrevHash: event.PrevHash,
		}
		raw, _ := json.Marshal(payload)
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	if event.HashVersion >= legacyAuditHashVersion {
		payload := auditHashPayloadV2{
			CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339Nano), RequestID: event.RequestID,
			Actor: event.Actor, Action: event.Action, Target: event.Target,
			HashVersion: event.HashVersion, PrevHash: event.PrevHash,
		}
		raw, _ := json.Marshal(payload)
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	payload := auditHashPayload{
		CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339Nano), RequestID: event.RequestID,
		Actor: event.Actor, Action: event.Action, Target: event.Target, Result: event.Result,
		HTTPStatus: event.HTTPStatus, PrevHash: event.PrevHash,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func calculateAuditOutcomeHash(auditHash, status string, httpStatus int, completedAt time.Time) string {
	payload := auditOutcomeHashPayload{
		AuditHash: auditHash, Status: status, HTTPStatus: httpStatus,
		CompletedAt: completedAt.UTC().Format(time.RFC3339Nano),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func verifyAuditEvents(events []models.AuditEvent) (bool, uint, string, int) {
	previousHash := ""
	pendingCount := 0
	for _, event := range events {
		if event.PrevHash != previousHash {
			return false, event.ID, "prevHash mismatch", pendingCount
		}
		if calculateAuditHash(event) != event.Hash {
			return false, event.ID, "hash mismatch", pendingCount
		}
		if event.HashVersion == systemAuditHashVersion {
			if event.Actor == "" || event.PayloadHash == "" || event.Status == "" || event.CompletedAt == nil || event.OutcomeHash == "" {
				return false, event.ID, "completed system outcome is incomplete", pendingCount
			}
		} else if event.HashVersion >= legacyAuditHashVersion {
			if event.HashVersion >= auditHashVersion && event.PayloadHash == "" {
				return false, event.ID, "request payload hash is missing", pendingCount
			}
			if event.Status == "PENDING" {
				if event.CompletedAt != nil || event.OutcomeHash != "" || event.HTTPStatus != 0 {
					return false, event.ID, "pending outcome contains completion fields", pendingCount
				}
				pendingCount++
			} else {
				if event.Status == "" || event.CompletedAt == nil || event.OutcomeHash == "" {
					return false, event.ID, "completed outcome is incomplete", pendingCount
				}
				if event.Result != "" && event.Result != event.Status {
					return false, event.ID, "result/status mismatch", pendingCount
				}
				expected := calculateAuditOutcomeHash(event.Hash, event.Status, event.HTTPStatus, *event.CompletedAt)
				if subtle.ConstantTimeCompare([]byte(expected), []byte(event.OutcomeHash)) != 1 {
					return false, event.ID, "outcome hash mismatch", pendingCount
				}
			}
		}
		previousHash = event.Hash
	}
	return true, 0, "", pendingCount
}

// verifyAuditEventsWithEvidence verifies both the linked audit chain and Paper
// fill/baseline payloads against their persisted ledger rows.
func verifyAuditEventsWithEvidence(db *gorm.DB, events []models.AuditEvent) (bool, uint, string, int) {
	valid, brokenAt, reason, pendingCount := verifyAuditEvents(events)
	if !valid || db == nil {
		return valid, brokenAt, reason, pendingCount
	}
	auditedFillIDs := make(map[string]struct{})
	for _, event := range events {
		if strings.HasPrefix(event.Target, "paper-fill:") {
			fillID := strings.TrimPrefix(event.Target, "paper-fill:")
			if _, exists := auditedFillIDs[fillID]; exists {
				return false, event.ID, "paper fill evidence is duplicated in audit chain", pendingCount
			}
			auditedFillIDs[fillID] = struct{}{}
			var fill models.PaperFill
			if err := db.Where("fill_id = ?", fillID).First(&fill).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return false, event.ID, "paper fill evidence is missing", pendingCount
				}
				return false, event.ID, "paper fill evidence cannot be read: " + err.Error(), pendingCount
			}
			expected := paperSystemAuditHash(fill)
			if subtle.ConstantTimeCompare([]byte(expected), []byte(event.PayloadHash)) != 1 {
				return false, event.ID, "paper fill payload hash mismatch", pendingCount
			}
			continue
		}
		if event.Action == paperDailyBaselineAuditAction || strings.HasPrefix(event.Target, paperDailyBaselineAuditTargetPrefix) {
			var baseline models.PaperDailyEquityBaseline
			var evidenceErr error
			if strings.HasPrefix(event.Target, paperDailyBaselineAuditTargetPrefix) {
				rawID := strings.TrimPrefix(event.Target, paperDailyBaselineAuditTargetPrefix)
				baselineID, err := strconv.ParseUint(rawID, 10, 64)
				if err != nil || baselineID == 0 {
					return false, event.ID, "paper daily equity baseline reference is invalid", pendingCount
				}
				evidenceErr = db.First(&baseline, uint(baselineID)).Error
			} else {
				currency, marketDate, ok := strings.Cut(event.Target, ":")
				if !ok || strings.TrimSpace(currency) == "" || strings.TrimSpace(marketDate) == "" {
					return false, event.ID, "legacy paper daily equity baseline reference is invalid", pendingCount
				}
				evidenceErr = db.Where("currency = ? AND market_date = ?", currency, marketDate).First(&baseline).Error
			}
			if evidenceErr != nil {
				if errors.Is(evidenceErr, gorm.ErrRecordNotFound) {
					return false, event.ID, "paper daily equity baseline evidence is missing", pendingCount
				}
				return false, event.ID, "paper daily equity baseline evidence cannot be read: " + evidenceErr.Error(), pendingCount
			}
			if expected := paperSystemAuditHash(paperDailyBaselineAuditPayload(baseline)); subtle.ConstantTimeCompare([]byte(expected), []byte(event.PayloadHash)) != 1 {
				return false, event.ID, "paper daily equity baseline payload hash mismatch", pendingCount
			}
			if expected := paperSystemAuditHash(paperDailyBaselineAuditOutcome(baseline)); subtle.ConstantTimeCompare([]byte(expected), []byte(event.OutcomeHash)) != 1 {
				return false, event.ID, "paper daily equity baseline outcome hash mismatch", pendingCount
			}
			continue
		}
		const legacyPrefix = "paper-order:"
		const legacySuffix = "#fillQuote"
		if strings.HasPrefix(event.Target, legacyPrefix) && strings.HasSuffix(event.Target, legacySuffix) {
			clientOrderID := strings.TrimSuffix(strings.TrimPrefix(event.Target, legacyPrefix), legacySuffix)
			var order models.PaperOrder
			if err := db.Where("client_order_id = ?", clientOrderID).First(&order).Error; err != nil {
				return false, event.ID, "legacy paper fill quote evidence is missing", pendingCount
			}
			matched := false
			for version := uint64(1); version <= order.Version; version++ {
				if subtle.ConstantTimeCompare([]byte(paperSystemAuditHash(paperFillQuoteAuditPayloadAtVersion(order, version))), []byte(event.PayloadHash)) == 1 {
					matched = true
					break
				}
			}
			if !matched {
				return false, event.ID, "legacy paper fill quote payload hash mismatch", pendingCount
			}
		}
	}
	if reason := verifyPaperLedgerIntegrity(db, auditedFillIDs); reason != "" {
		return false, 0, reason, pendingCount
	}
	return true, 0, "", pendingCount
}

// appendCompletedSystemAuditTx atomically joins an automatic Paper transition to
// the same append-only audit chain used by guarded admin operations. Version 3
// includes both payload and outcome hashes in the immutable chain hash.
func appendCompletedSystemAuditTx(tx *gorm.DB, requestID, action, target, payloadHash, outcomeHash string, completedAt time.Time) error {
	return appendCompletedSystemAuditActorTx(tx, requestID, "system:paper-stop-scanner", action, target, payloadHash, outcomeHash, completedAt)
}

func appendCompletedSystemAuditActorTx(tx *gorm.DB, requestID, actor, action, target, payloadHash, outcomeHash string, completedAt time.Time) error {
	if tx == nil || payloadHash == "" || outcomeHash == "" {
		return errors.New("system audit payload is incomplete")
	}
	var previous models.AuditEvent
	err := tx.Order("id desc").First(&previous).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	event := models.AuditEvent{
		CreatedAt: completedAt.UTC(), CompletedAt: func() *time.Time { value := completedAt.UTC(); return &value }(),
		RequestID: requestID, Actor: actor, Action: action, Target: target,
		Status: "SUCCEEDED", Result: "SUCCEEDED", HTTPStatus: http.StatusOK,
		PayloadHash: payloadHash, OutcomeHash: outcomeHash, HashVersion: systemAuditHashVersion,
	}
	if err == nil {
		event.PrevHash = previous.Hash
	}
	event.Hash = calculateAuditHash(event)
	return tx.Create(&event).Error
}

func (h *Handler) ListAuditEvents(c *gin.Context) {
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 500 {
		limit = 500
	}
	var events []models.AuditEvent
	if err := database.DB.Order("id desc").Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for index := range events {
		if events[index].Status == "" {
			events[index].Status = events[index].Result
		}
	}
	pendingCount, err := countPendingAuditOutcomes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"events": events, "count": len(events), "pendingCount": pendingCount,
		"requiresAttention": pendingCount > 0,
	})
}

func (h *Handler) VerifyAuditChain(c *gin.Context) {
	var events []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	valid, brokenAt, reason, pendingCount := verifyAuditEventsWithEvidence(database.DB, events)
	status := http.StatusOK
	if !valid {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{
		"valid": valid, "count": len(events), "brokenAtId": brokenAt,
		"reason": reason, "algorithm": "SHA-256 linked records with hashed outcomes, persisted Paper evidence, and replayed ledger invariants",
		"pendingCount": pendingCount, "requiresAttention": pendingCount > 0,
	})
}

func countPendingAuditOutcomes() (int64, error) {
	if database.DB == nil {
		return 0, errors.New("audit store unavailable")
	}
	var count int64
	err := database.DB.Model(&models.AuditEvent{}).
		Where("hash_version >= ? AND status = ?", auditHashVersion, "PENDING").
		Count(&count).Error
	return count, err
}

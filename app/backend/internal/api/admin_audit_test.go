package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:audit-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AuditEvent{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperAccount{}, &models.PaperPosition{}); err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	return db
}

func performAdminRequest(router http.Handler, method, path, authorization string) *httptest.ResponseRecorder {
	return performAdminRequestWithID(router, method, path, authorization, "")
}

func performAdminRequestWithID(router http.Handler, method, path, authorization, requestID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if requestID != "" {
		req.Header.Set(requestIDHeader, requestID)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdminWriteGuardFailsClosedWithoutSideEffectsAndAudits(t *testing.T) {
	db := setupAuditTestDB(t)
	gin.SetMode(gin.TestMode)
	calls := 0
	secret := "do-not-store-this-token"
	cfg := &config.Config{AdminToken: secret}
	router := gin.New()
	router.POST("/write", adminWriteGuard(cfg, "test.write"), func(c *gin.Context) {
		calls++
		c.Status(http.StatusNoContent)
	})

	cfg.AdminToken = ""
	if got := performAdminRequest(router, http.MethodPost, "/write", "").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured status=%d", got)
	}
	cfg.AdminToken = secret
	if got := performAdminRequest(router, http.MethodPost, "/write", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d", got)
	}
	if got := performAdminRequest(router, http.MethodPost, "/write", "Bearer wrong-token").Code; got != http.StatusForbidden {
		t.Fatalf("wrong token status=%d", got)
	}
	allowed := performAdminRequest(router, http.MethodPost, "/write", "Bearer "+secret)
	if allowed.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("allowed status=%d calls=%d", allowed.Code, calls)
	}

	var events []models.AuditEvent
	if err := db.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("audit count=%d want 4", len(events))
	}
	valid, _, reason, pendingCount := verifyAuditEvents(events)
	if !valid {
		t.Fatalf("audit chain invalid: %s", reason)
	}
	if pendingCount != 0 {
		t.Fatalf("completed requests must not remain pending: %d", pendingCount)
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "wrong-token") {
		t.Fatal("audit events must not contain credentials")
	}
	if events[0].Status != "DENIED_NOT_CONFIGURED" || events[1].Status != "DENIED_MISSING_TOKEN" || events[2].Status != "DENIED_INVALID_TOKEN" || events[3].Status != "SUCCEEDED" {
		t.Fatalf("unexpected results: %+v", events)
	}
	for _, event := range events {
		if event.CompletedAt == nil || event.OutcomeHash == "" {
			t.Fatalf("completed outcome is incomplete: %+v", event)
		}
	}
}

func TestLocalAdminSessionIsLoopbackOnlyAndWorksWithoutExposingToken(t *testing.T) {
	setupAuditTestDB(t)
	cfg := &config.Config{AdminToken: "server-only-secret"}
	router := gin.New()
	router.Any("/session", localAdminSessionHandler(cfg))
	router.POST("/write", adminWriteGuard(cfg, "test.write"), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	remote := httptest.NewRequest(http.MethodPost, "/session", nil)
	remote.Host = "127.0.0.1:8831"
	remote.RemoteAddr = "192.0.2.1:1234"
	remoteResult := httptest.NewRecorder()
	router.ServeHTTP(remoteResult, remote)
	if remoteResult.Code != http.StatusForbidden {
		t.Fatalf("remote session status=%d", remoteResult.Code)
	}

	local := httptest.NewRequest(http.MethodPost, "/session", nil)
	local.Host = "127.0.0.1:8831"
	local.RemoteAddr = "127.0.0.1:1234"
	local.Header.Set("Origin", "http://127.0.0.1:8831")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, local)
	if created.Code != http.StatusOK || strings.Contains(created.Body.String(), cfg.AdminToken) {
		t.Fatalf("local session response=%d body=%s", created.Code, created.Body.String())
	}
	cookies := created.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie is not hardened: %+v", cookies)
	}

	validate := httptest.NewRequest(http.MethodGet, "/session", nil)
	validate.Host = "127.0.0.1:8831"
	validate.RemoteAddr = "127.0.0.1:1234"
	validate.Header.Set("Sec-Fetch-Site", "same-origin")
	validate.AddCookie(cookies[0])
	validated := httptest.NewRecorder()
	router.ServeHTTP(validated, validate)
	if validated.Code != http.StatusOK {
		t.Fatalf("valid local session status=%d body=%s", validated.Code, validated.Body.String())
	}

	localhost := httptest.NewRequest(http.MethodPost, "/session", nil)
	localhost.Host = "localhost:8831"
	localhost.RemoteAddr = "127.0.0.1:1234"
	localhost.Header.Set("Origin", "http://localhost:8831")
	localhostResult := httptest.NewRecorder()
	router.ServeHTTP(localhostResult, localhost)
	if localhostResult.Code != http.StatusOK {
		t.Fatalf("localhost session status=%d body=%s", localhostResult.Code, localhostResult.Body.String())
	}

	crossOrigin := httptest.NewRequest(http.MethodPost, "/session", nil)
	crossOrigin.Host = "127.0.0.1:8831"
	crossOrigin.RemoteAddr = "127.0.0.1:1234"
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	crossOriginResult := httptest.NewRecorder()
	router.ServeHTTP(crossOriginResult, crossOrigin)
	if crossOriginResult.Code != http.StatusForbidden {
		t.Fatalf("cross-origin local session status=%d", crossOriginResult.Code)
	}

	write := httptest.NewRequest(http.MethodPost, "/write", strings.NewReader(`{}`))
	write.Host = "127.0.0.1:8831"
	write.RemoteAddr = "127.0.0.1:1234"
	write.AddCookie(cookies[0])
	result := httptest.NewRecorder()
	router.ServeHTTP(result, write)
	if result.Code != http.StatusNoContent {
		t.Fatalf("local session did not authorize audited write: %d %s", result.Code, result.Body.String())
	}
}

func TestAdminWriteGuardCompletesOnePersistentOutcome(t *testing.T) {
	db := setupAuditTestDB(t)
	pendingVisibleBeforeBusiness := false
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		var count int64
		if err := db.Model(&models.AuditEvent{}).
			Where("request_id = ? AND status = ? AND completed_at IS NULL", c.GetHeader(requestIDHeader), "PENDING").
			Count(&count).Error; err == nil && count == 1 {
			pendingVisibleBeforeBusiness = true
		}
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
	})

	response := performAdminRequestWithID(router, http.MethodPost, "/write", "Bearer secret", "client-request")
	if response.Code != http.StatusAccepted || response.Header().Get(auditOutcomeHeader) != "SUCCEEDED" {
		t.Fatalf("status=%d audit=%q body=%s", response.Code, response.Header().Get(auditOutcomeHeader), response.Body.String())
	}
	if !pendingVisibleBeforeBusiness {
		t.Fatal("persistent PENDING outcome was not visible before business handler")
	}
	var event models.AuditEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.RequestID != "client-request" || event.Status != "SUCCEEDED" || event.HTTPStatus != http.StatusAccepted || event.CompletedAt == nil || event.OutcomeHash == "" {
		t.Fatalf("unexpected completed outcome: %+v", event)
	}
	valid, _, reason, pendingCount := verifyAuditEvents([]models.AuditEvent{event})
	if !valid || pendingCount != 0 {
		t.Fatalf("valid=%v pending=%d reason=%s", valid, pendingCount, reason)
	}
}

func TestReconcilePendingAuditOutcomesFinalizesPreviousProcessRequests(t *testing.T) {
	db := setupAuditTestDB(t)
	pending, err := createPendingAuditOutcome("interrupted-request", "local-session", "market.refresh", "/api/market/refresh")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ReconcilePendingAuditOutcomes()
	if err != nil || resolved != 1 {
		t.Fatalf("resolved=%d err=%v", resolved, err)
	}

	var event models.AuditEvent
	if err := db.First(&event, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if event.Status != "FAILED" || event.Result != "FAILED" || event.HTTPStatus != http.StatusServiceUnavailable || event.CompletedAt == nil || event.OutcomeHash == "" {
		t.Fatalf("reconciled event=%+v", event)
	}
	valid, brokenAt, reason, pendingCount := verifyAuditEvents([]models.AuditEvent{event})
	if !valid || brokenAt != 0 || reason != "" || pendingCount != 0 {
		t.Fatalf("valid=%v brokenAt=%d reason=%q pending=%d", valid, brokenAt, reason, pendingCount)
	}

	resolved, err = ReconcilePendingAuditOutcomes()
	if err != nil || resolved != 0 {
		t.Fatalf("second reconcile resolved=%d err=%v", resolved, err)
	}
}

func TestAdminWriteGuardRecordsBusinessRejection(t *testing.T) {
	db := setupAuditTestDB(t)
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "business rule"})
	})

	response := performAdminRequest(router, http.MethodPost, "/write", "Bearer secret")
	if response.Code != http.StatusUnprocessableEntity || response.Header().Get(auditOutcomeHeader) != "REJECTED" {
		t.Fatalf("status=%d audit=%q body=%s", response.Code, response.Header().Get(auditOutcomeHeader), response.Body.String())
	}
	var event models.AuditEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Status != "REJECTED" || event.HTTPStatus != http.StatusUnprocessableEntity || event.CompletedAt == nil {
		t.Fatalf("unexpected rejected outcome: %+v", event)
	}
}

func TestAdminWriteGuardLeavesPendingWhenFinalUpdateFails(t *testing.T) {
	db := setupAuditTestDB(t)
	if err := db.Exec(`CREATE TRIGGER fail_audit_finalize
		BEFORE UPDATE OF status ON audit_events
		WHEN OLD.status = 'PENDING' AND NEW.status <> 'PENDING'
		BEGIN SELECT RAISE(ABORT, 'forced audit finalize failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		calls++
		c.JSON(http.StatusCreated, gin.H{"created": true})
	})

	response := performAdminRequest(router, http.MethodPost, "/write", "Bearer secret")
	if response.Code != http.StatusServiceUnavailable || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, calls, response.Body.String())
	}
	if response.Header().Get(auditOutcomeHeader) != "PENDING" || response.Header().Get(auditErrorHeader) != "finalize-failed" {
		t.Fatalf("audit headers not visible: %+v", response.Header())
	}
	var event models.AuditEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Status != "PENDING" || event.CompletedAt != nil || event.HTTPStatus != 0 || event.OutcomeHash != "" {
		t.Fatalf("failed final update must preserve pending row: %+v", event)
	}
	valid, _, reason, pendingCount := verifyAuditEvents([]models.AuditEvent{event})
	if !valid || pendingCount != 1 {
		t.Fatalf("valid=%v pending=%d reason=%s", valid, pendingCount, reason)
	}
}

func TestAdminWriteGuardKeepsRequestIDsUnique(t *testing.T) {
	db := setupAuditTestDB(t)
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	first := performAdminRequestWithID(router, http.MethodPost, "/write", "Bearer secret", "reused-id")
	second := performAdminRequestWithID(router, http.MethodPost, "/write", "Bearer secret", "reused-id")
	if first.Code != http.StatusNoContent || second.Code != http.StatusConflict {
		t.Fatalf("duplicate request status first=%d second=%d", first.Code, second.Code)
	}
	var count int64
	if err := db.Model(&models.AuditEvent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestAdminWriteAuditBindsBodyAndQuery(t *testing.T) {
	db := setupAuditTestDB(t)
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		var payload map[string]any
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/write?mode=strict", strings.NewReader(`{"symbol":"AAPL"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(requestIDHeader, "payload-bound")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var event models.AuditEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.PayloadHash == "" {
		t.Fatal("admin write audit did not bind request payload")
	}
	event.PayloadHash = strings.Repeat("0", 64)
	valid, _, _, _ := verifyAuditEvents([]models.AuditEvent{event})
	if valid {
		t.Fatal("payload hash tampering was not detected")
	}
}

func TestAdminWriteGuardDeniesWhenAuditStoreCannotWrite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:audit-missing-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	calls := 0
	router := gin.New()
	router.POST("/write", adminWriteGuard(&config.Config{AdminToken: "secret"}, "test.write"), func(c *gin.Context) {
		calls++
		c.Status(http.StatusNoContent)
	})
	response := performAdminRequest(router, http.MethodPost, "/write", "Bearer secret")
	if response.Code != http.StatusServiceUnavailable || calls != 0 {
		t.Fatalf("audit failure must deny before handler: status=%d calls=%d", response.Code, calls)
	}
}

func TestAuditChainDetectsTampering(t *testing.T) {
	db := setupAuditTestDB(t)
	for _, result := range []string{"DENIED_MISSING_TOKEN", "ALLOWED"} {
		if err := appendAuditEvent(models.AuditEvent{RequestID: "request-1", Actor: "admin", Action: "test", Target: "/write", Result: result, HTTPStatus: 200}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&models.AuditEvent{}).Where("id = ?", 1).Update("result", "TAMPERED").Error; err != nil {
		t.Fatal(err)
	}
	var events []models.AuditEvent
	db.Order("id asc").Find(&events)
	valid, brokenAt, _, _ := verifyAuditEvents(events)
	if valid || brokenAt != 1 {
		t.Fatalf("tampering not detected: valid=%v brokenAt=%d", valid, brokenAt)
	}
}

func TestAuditChainDetectsCompletedOutcomeTampering(t *testing.T) {
	db := setupAuditTestDB(t)
	pending, err := createPendingAuditOutcome("request-v2", "admin", "test.write", "/write")
	if err != nil {
		t.Fatal(err)
	}
	if err := finalizeAuditOutcome(pending.ID, "SUCCEEDED", http.StatusNoContent); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AuditEvent{}).Where("id = ?", pending.ID).
		Updates(map[string]any{"status": "FAILED", "result": "FAILED"}).Error; err != nil {
		t.Fatal(err)
	}
	var event models.AuditEvent
	if err := db.First(&event, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	valid, brokenAt, reason, _ := verifyAuditEvents([]models.AuditEvent{event})
	if valid || brokenAt != pending.ID || reason != "outcome hash mismatch" {
		t.Fatalf("outcome tampering not detected: valid=%v brokenAt=%d reason=%q", valid, brokenAt, reason)
	}
}

func TestAllSensitivePaperAndSideEffectRoutesRequireAdmin(t *testing.T) {
	setupAuditTestDB(t)
	cfg := &config.Config{AdminToken: "route-secret"}
	router := SetupRouter(cfg, nil)

	protected := []struct{ method, path string }{
		{http.MethodPost, "/api/analysis/start"}, {http.MethodPost, "/api/analysis/evidence-only"}, {http.MethodPost, "/api/analysis/stop"},
		{http.MethodPost, "/api/analysis/batch"}, {http.MethodPost, "/api/chat/ask"},
		{http.MethodPut, "/api/config"}, {http.MethodPost, "/api/config/test"},
		{http.MethodPost, "/api/trades"}, {http.MethodPost, "/api/events"},
		{http.MethodPost, "/api/whales/sync"},
		{http.MethodPost, "/api/market/refresh"}, {http.MethodPost, "/api/panel-summary/refresh"},
		{http.MethodPost, "/api/gpu-prices/refresh"}, {http.MethodPost, "/api/gpu-prices/reload-config"},
		{http.MethodPost, "/api/etf/refresh"},
		{http.MethodPost, "/api/paper-orders"}, {http.MethodPost, "/api/paper-orders/order-1/cancel"},
		{http.MethodPost, "/api/paper-orders/order-1/simulated-fill"},
		{http.MethodPost, "/api/paper-orders/validate"},
		{http.MethodGet, "/api/paper-orders"}, {http.MethodGet, "/api/paper-orders/account"},
		{http.MethodGet, "/api/paper-orders/portfolio-risk"},
		{http.MethodGet, "/api/paper-orders/scheduler"},
		{http.MethodGet, "/api/admin/audit"}, {http.MethodGet, "/api/admin/audit/verify"},
		{http.MethodGet, "/api/analysis/status"}, {http.MethodGet, "/api/analysis/history"},
		{http.MethodGet, "/api/analysis/batch/job-1"}, {http.MethodGet, "/api/config"},
		{http.MethodGet, "/api/system/status"}, {http.MethodGet, "/api/trades"},
		{http.MethodGet, "/api/stats"}, {http.MethodGet, "/api/events"},
		{http.MethodGet, "/ws"},
	}
	for _, route := range protected {
		response := performAdminRequest(router, route.method, route.path, "")
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status=%d body=%s", route.method, route.path, response.Code, response.Body.String())
		}
	}
	cfg.AdminToken = ""
	for _, route := range protected {
		response := performAdminRequest(router, route.method, route.path, "")
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("unconfigured %s %s status=%d body=%s", route.method, route.path, response.Code, response.Body.String())
		}
	}
	cfg.AdminToken = "route-secret"
	invalidValidate := performAdminRequest(router, http.MethodPost, "/api/paper-orders/validate", "Bearer wrong-secret")
	if invalidValidate.Code != http.StatusForbidden {
		t.Fatalf("invalid Bearer reached paper validate, status=%d body=%s", invalidValidate.Code, invalidValidate.Body.String())
	}
	passedGuard := performAdminRequest(router, http.MethodPost, "/api/paper-orders", "Bearer route-secret")
	if passedGuard.Code != http.StatusBadRequest {
		t.Fatalf("correct Bearer should reach paper business validation, status=%d body=%s", passedGuard.Code, passedGuard.Body.String())
	}

	response := performAdminRequest(router, http.MethodPost, "/api/paper-orders/validate", "Bearer route-secret")
	if response.Code != http.StatusOK {
		t.Fatalf("authorized paper validate should reach business validation, status=%d body=%s", response.Code, response.Body.String())
	}
	var paperCount int64
	if err := database.DB.Model(&models.PaperOrder{}).Count(&paperCount).Error; err != nil || paperCount != 0 {
		t.Fatalf("denied paper submit had side effects: count=%d err=%v", paperCount, err)
	}
}

func TestProtectedReadDoesNotMutateAuditChain(t *testing.T) {
	db := setupAuditTestDB(t)
	pending, err := createPendingAuditOutcome("pending-request", "admin", "test.write", "/write")
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != "PENDING" {
		t.Fatalf("unexpected pending event: %+v", pending)
	}
	router := SetupRouter(&config.Config{AdminToken: "read-secret"}, nil)
	verifyResponse := performAdminRequest(router, http.MethodGet, "/api/admin/audit/verify", "Bearer read-secret")
	listResponse := performAdminRequest(router, http.MethodGet, "/api/admin/audit", "Bearer read-secret")
	if verifyResponse.Code != http.StatusOK || listResponse.Code != http.StatusOK {
		t.Fatalf("protected read failed: verify=%d list=%d", verifyResponse.Code, listResponse.Code)
	}
	for name, response := range map[string]*httptest.ResponseRecorder{"verify": verifyResponse, "list": listResponse} {
		var body struct {
			PendingCount      int  `json:"pendingCount"`
			RequiresAttention bool `json:"requiresAttention"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.PendingCount != 1 || !body.RequiresAttention {
			t.Fatalf("%s did not expose pending outcome: body=%s err=%v", name, response.Body.String(), err)
		}
	}
	var count int64
	if err := db.Model(&models.AuditEvent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("protected read mutated audit chain: count=%d err=%v", count, err)
	}
}

func TestLiveMirrorDoesNotBypassWriteAuthentication(t *testing.T) {
	setupAuditTestDB(t)
	t.Setenv("STOCKGOD_REPLAY", "")
	t.Setenv("STOCKGOD_LIVE_MIRROR", "true")
	router := SetupRouter(&config.Config{AdminToken: "mirror-secret"}, nil)
	response := performAdminRequest(router, http.MethodPost, "/api/remote-write", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("mirror write bypassed authentication: status=%d body=%s", response.Code, response.Body.String())
	}
}

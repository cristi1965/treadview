package api

import (
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
)

const adminSessionCookie = "stockgod_admin_session"

var localAdminSessions = struct {
	sync.Mutex
	values map[string]time.Time
}{values: make(map[string]time.Time)}

func localAdminSessionHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isLoopbackRequest(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "local admin sessions require a loopback connection"})
			return
		}
		if cfg == nil || strings.TrimSpace(cfg.AdminToken) == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin authentication is not configured"})
			return
		}
		if !sameOriginBrowserRequest(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "local admin sessions require a same-origin browser request"})
			return
		}
		switch c.Request.Method {
		case http.MethodGet:
			if !validLocalAdminSession(c) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "local admin session expired"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
		case http.MethodPost:
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "cannot create local admin session"})
				return
			}
			value := base64.RawURLEncoding.EncodeToString(raw)
			expires := time.Now().Add(time.Hour)
			localAdminSessions.Lock()
			localAdminSessions.values[value] = expires
			localAdminSessions.Unlock()
			http.SetCookie(c.Writer, &http.Cookie{Name: adminSessionCookie, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(time.Hour.Seconds())})
			c.JSON(http.StatusOK, gin.H{"authenticated": true, "expiresAt": expires.UTC().Format(time.RFC3339)})
		case http.MethodDelete:
			if cookie, err := c.Cookie(adminSessionCookie); err == nil {
				localAdminSessions.Lock()
				delete(localAdminSessions.values, cookie)
				localAdminSessions.Unlock()
			}
			http.SetCookie(c.Writer, &http.Cookie{Name: adminSessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
			c.JSON(http.StatusOK, gin.H{"authenticated": false})
		default:
			c.Status(http.StatusMethodNotAllowed)
		}
	}
}

func sameOriginBrowserRequest(request *http.Request) bool {
	fetchSite := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")))
	if fetchSite == "cross-site" {
		return false
	}
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin != "" {
		parsed, err := url.Parse(origin)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Host, request.Host)
	}
	if referer := strings.TrimSpace(request.Header.Get("Referer")); referer != "" {
		parsed, err := url.Parse(referer)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Host, request.Host)
	}
	return fetchSite == "same-origin"
}

func isLoopbackRequest(request *http.Request) bool {
	host := request.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	hostIsLoopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		hostIsLoopback = ip.IsLoopback()
	}
	remoteHost, _, err := net.SplitHostPort(request.RemoteAddr)
	remoteIP := net.ParseIP(remoteHost)
	return hostIsLoopback && err == nil && remoteIP != nil && remoteIP.IsLoopback()
}

func validLocalAdminSession(c *gin.Context) bool {
	if !isLoopbackRequest(c.Request) {
		return false
	}
	value, err := c.Cookie(adminSessionCookie)
	if err != nil || value == "" {
		return false
	}
	localAdminSessions.Lock()
	defer localAdminSessions.Unlock()
	expires, ok := localAdminSessions.values[value]
	if !ok || time.Now().After(expires) {
		delete(localAdminSessions.values, value)
		return false
	}
	return true
}

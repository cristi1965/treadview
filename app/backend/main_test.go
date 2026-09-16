package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressStaticAssetsOnly(t *testing.T) {
	payload := strings.Repeat("const trusted = true;", 100)
	handler := compressStaticAssets(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = io.WriteString(w, payload)
	}))

	assetRequest := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetRequest.Header.Set("Accept-Encoding", "br, gzip")
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, assetRequest)
	if asset.Header().Get("Content-Encoding") != "gzip" || asset.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("asset was not gzip encoded: headers=%v", asset.Header())
	}
	reader, err := gzip.NewReader(asset.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != payload {
		t.Fatal("compressed asset body changed")
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	apiRequest.Header.Set("Accept-Encoding", "gzip")
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, apiRequest)
	if api.Header().Get("Content-Encoding") != "" || api.Body.String() != payload {
		t.Fatalf("non-static response was changed: headers=%v", api.Header())
	}
}

package dataflows

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var safeEvidencePathPart = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// PersistPayloadArtifact stores the exact tool response outside the default API payload.
func PersistPayloadArtifact(resultsDir, runID, evidenceID string, invocation ToolInvocation) (string, int64, error) {
	if resultsDir == "" || !safeEvidencePathPart.MatchString(runID) || !safeEvidencePathPart.MatchString(evidenceID) {
		return "", 0, fmt.Errorf("invalid evidence artifact path")
	}
	raw := []byte(invocation.RawPayload)
	if len(raw) == 0 {
		return "", 0, fmt.Errorf("empty evidence payload")
	}
	sum := sha256.Sum256(raw)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != invocation.ContentHash {
		return "", 0, fmt.Errorf("payload hash mismatch")
	}
	rel := filepath.Join("evidence", runID, evidenceID+".payload.gz")
	path := filepath.Join(resultsDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", 0, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", 0, err
	}
	zw := gzip.NewWriter(file)
	_, writeErr := zw.Write(raw)
	closeGzipErr := zw.Close()
	closeFileErr := file.Close()
	if writeErr != nil {
		return "", 0, writeErr
	}
	if closeGzipErr != nil {
		return "", 0, closeGzipErr
	}
	if closeFileErr != nil {
		return "", 0, closeFileErr
	}
	return filepath.ToSlash(rel), int64(len(raw)), nil
}

// ReadPayloadArtifact resolves only the controlled run/evidence path and verifies the stored hash.
func ReadPayloadArtifact(resultsDir, runID, evidenceID, expectedHash string) ([]byte, error) {
	if resultsDir == "" || !safeEvidencePathPart.MatchString(runID) || !safeEvidencePathPart.MatchString(evidenceID) {
		return nil, fmt.Errorf("invalid evidence artifact path")
	}
	path := filepath.Join(resultsDir, "evidence", runID, evidenceID+".payload.gz")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	zr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, 32<<20))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if got := "sha256:" + hex.EncodeToString(sum[:]); expectedHash != "" && got != expectedHash {
		return nil, fmt.Errorf("payload hash mismatch")
	}
	return raw, nil
}

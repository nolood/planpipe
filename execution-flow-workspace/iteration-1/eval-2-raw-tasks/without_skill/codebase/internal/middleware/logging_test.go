package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	// Capture log output into a buffer.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0) // remove timestamps for easier assertion
	defer func() {
		log.SetOutput(nil)
		log.SetFlags(log.LstdFlags)
	}()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logLine := buf.String()

	if !strings.Contains(logLine, "POST") {
		t.Errorf("log should contain method POST, got: %s", logLine)
	}
	if !strings.Contains(logLine, "/test/path") {
		t.Errorf("log should contain path /test/path, got: %s", logLine)
	}
	if !strings.Contains(logLine, "201") {
		t.Errorf("log should contain status code 201, got: %s", logLine)
	}
	// Duration is always present — just check there's a time unit suffix.
	if !strings.Contains(logLine, "s") {
		t.Errorf("log should contain duration, got: %s", logLine)
	}
}

func TestRequestLoggerDefaultStatus(t *testing.T) {
	// When handler does not call WriteHeader explicitly, status should be 200.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(nil)
		log.SetFlags(log.LstdFlags)
	}()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logLine := buf.String()
	if !strings.Contains(logLine, "200") {
		t.Errorf("log should contain default status code 200, got: %s", logLine)
	}
}

package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	}()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, "POST") {
		t.Errorf("log output should contain method POST, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/test/path") {
		t.Errorf("log output should contain path /test/path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "201") {
		t.Errorf("log output should contain status code 201, got: %s", logOutput)
	}
	// Duration is always present as the last field (e.g., "1.234us" or "0s")
	// We just check the log line is non-empty and has the expected fields above.
}

func TestRequestLoggerDefaultStatusCode(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	}()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No explicit WriteHeader call -- should default to 200
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/default", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, "GET") {
		t.Errorf("log output should contain method GET, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/default") {
		t.Errorf("log output should contain path /default, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "200") {
		t.Errorf("log output should contain status code 200, got: %s", logOutput)
	}
}

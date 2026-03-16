# Implementation Summary

## Task 1: Health Check Endpoint

**Files created:**
- `internal/health/handler.go` — `HandleHealth` function returns JSON `{"status": "ok", "version": "1.0.0"}` with Content-Type `application/json`
- `internal/health/handler_test.go` — Tests status code, content type, and response body fields

**Files modified:**
- `main.go` — Added import for `health` package, registered `GET /health` route

## Task 2: Request Logging Middleware

**Files created:**
- `internal/middleware/logging.go` — `RequestLogger` middleware logs method, path, status code, and duration using the standard `log` package. Uses a `statusWriter` wrapper to capture the response status code.
- `internal/middleware/logging_test.go` — Two tests: one verifying all four fields are logged with an explicit status code (201), one verifying the default 200 status when `WriteHeader` is not called explicitly.

**Files modified:**
- `main.go` — Added import for `middleware` package, applied `r.Use(middleware.RequestLogger)` to the chi router before all route registrations

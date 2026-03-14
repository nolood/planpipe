# Tasks

## 1. Add health check endpoint

Add a `/health` endpoint to the Go app that returns JSON `{"status": "ok", "version": "1.0.0"}`. Should be registered in `main.go` alongside the existing routes.

Done when:
- GET /health returns 200 with correct JSON body
- Endpoint is registered in main.go
- Has a basic test

## 2. Add request logging middleware

Create a middleware that logs method, path, status code, and duration for every HTTP request. Use the standard `log` package. Apply it to the chi router.

Done when:
- Middleware function exists in `internal/middleware/logging.go`
- Logs method, path, status code, duration
- Applied to the router in main.go
- Has a test that verifies logging output

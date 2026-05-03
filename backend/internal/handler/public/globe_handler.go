// Package public: GlobeHandler exposes the live-globe endpoints.
//
// Three endpoints are mounted under /api/public/globe:
//
//	GET /snapshot    one-shot most-recent frame (REST, JSON)
//	GET /summary     24h / lifetime rollup (REST, JSON, cached 30s)
//	GET /stream      Server-Sent Events feed of frames every 5 minutes
//
// All three are anonymous-safe: no raw IP, no email, no usernames are exposed
// in the public payload. Admin currently shares the same masked payload.
package public

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GlobeHandler is the handler bundle for the public globe endpoints.
type GlobeHandler struct {
	svc *service.GlobeService
}

// NewGlobeHandler is the wire constructor.
func NewGlobeHandler(svc *service.GlobeService) *GlobeHandler {
	return &GlobeHandler{svc: svc}
}

// Snapshot serves GET /api/public/globe/snapshot.
func (h *GlobeHandler) Snapshot(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Success(c, gin.H{})
		return
	}
	c.Header("Cache-Control", "public, max-age=1")
	response.Success(c, h.svc.SnapshotWithContext(c.Request.Context()))
}

// Summary serves GET /api/public/globe/summary.
func (h *GlobeHandler) Summary(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Success(c, gin.H{})
		return
	}
	sum, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to build summary")
		return
	}
	c.Header("Cache-Control", "public, max-age=15, s-maxage=15")
	response.Success(c, sum)
}

// Stream serves GET /api/public/globe/stream as Server-Sent Events.
//
// Implementation notes
//
//   - We rely on gin's underlying http.ResponseWriter.Flusher; this requires
//     the H2C handler we already enable in production (or HTTP/1.1 in dev).
//   - One subscription per HTTP request — when the client disconnects the
//     ctx.Done() select branch unsubscribes us cleanly.
//   - We send a heartbeat comment every 15s so reverse proxies / load
//     balancers don't kill an idle stream during low-traffic moments.
func (h *GlobeHandler) Stream(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Success(c, gin.H{})
		return
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		response.InternalError(c, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// Send a hello comment so proxies open the pipe.
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ch, unsub := h.svc.Subscribe()
	defer unsub()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case snap, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(snap)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", b); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// parsePosInt is a tiny helper for query params we may add later.
func parsePosInt(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

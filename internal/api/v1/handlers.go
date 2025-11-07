package v1

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"log/slog"
	"webpage-analyzer-service/internal/analyzer"
	"webpage-analyzer-service/internal/cache"
	"webpage-analyzer-service/internal/idempotency"
	"webpage-analyzer-service/internal/queue"
	"webpage-analyzer-service/pkg/ssrf"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	analyzer    *analyzer.Analyzer
	cache       cache.Cache
	idempotency idempotency.Store
	queue       queue.Queue
	logger      *slog.Logger
}

func NewHandler(a *analyzer.Analyzer, c cache.Cache, i idempotency.Store, q queue.Queue, l *slog.Logger) *Handler {
	return &Handler{analyzer: a, cache: c, idempotency: i, queue: q, logger: l}
}

func (h *Handler) Analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{"wpa1000", err.Error()})
		return
	}

	if existing, ok := h.idempotency.Get(req.RequestID); ok {
		h.logger.Info("duplicate request", "requestId", req.RequestID)
		_ = existing // suppress unused warning
		c.JSON(http.StatusConflict, ErrorResponse{"wpa1100", "duplicate requestId"})
		return
	}

	if !isValidURL(req.URL) {
		c.JSON(http.StatusBadRequest, ErrorResponse{"wpa1201", "invalid URL format"})
		return
	}

	parsed, _ := url.Parse(req.URL)
	if ok, err := ssrf.IsPublicHost(parsed.Hostname()); err != nil || !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{"wpa1300", "SSRF attempt blocked"})
		return
	}

	if cached, ok := h.cache.Get(req.URL); ok {
		h.logger.Info("cache hit", "url", req.URL)
		result := cached.(*analyzer.AnalysisResult)
		h.idempotency.Set(req.RequestID, result)
		c.JSON(http.StatusOK, result)
		return
	}

	job := analyzer.Job{
		RequestID: req.RequestID,
		UserID:    req.UserID,
		Channel:   req.Channel,
		Email:     req.Email,
		URL:       req.URL,
	}

	result, err := h.analyzer.Analyze(c.Request.Context(), job)
	if err != nil {
		code := "wpa1200"
		if strings.Contains(err.Error(), "404") {
			code = "wpa1200"
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{code, err.Error()})
		return
	}

	h.idempotency.Set(req.RequestID, result)
	h.cache.Set(req.URL, result, 10*time.Minute)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func isValidURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

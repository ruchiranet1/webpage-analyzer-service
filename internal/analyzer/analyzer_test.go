package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webpage-analyzer-service/internal/cache"
	"webpage-analyzer-service/internal/queue"

	"github.com/stretchr/testify/assert"
)

func TestAnalyze_CacheHit(t *testing.T) {
	mockCache := cache.NewMockCache()
	mockQueue := queue.NewMockQueue()
	a := New(mockCache, mockQueue)

	expected := &AnalysisResult{HTMLVersion: "HTML5"}
	mockCache.Set("https://mockcache.com", expected, time.Minute)

	job := Job{URL: "https://mockcache.com"}
	result, err := a.Analyze(context.Background(), job)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestAnalyze_FetchAndParse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Test</title></head><body><h1>A</h1></body></html>`))
	}))
	defer server.Close()

	mockCache := cache.NewMockCache()
	a := New(mockCache, queue.NewMockQueue())

	job := Job{URL: server.URL}
	result, err := a.Analyze(context.Background(), job)

	assert.NoError(t, err)
	assert.Equal(t, "HTML5", result.HTMLVersion)
	assert.Equal(t, "Test", result.PageTitle)
	assert.Equal(t, map[string]int{"h1": 1}, result.Headings)
}

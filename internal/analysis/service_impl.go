package analysis

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"webpage-analyzer-service/internal/constants"
	"webpage-analyzer-service/internal/domain"
)

// serviceImpl is the concrete implementation of the analysis.Service interface.
type serviceImpl struct {
	fetcher PageFetcher
	parser  PageParser
	checker LinkChecker
	logger  *slog.Logger
}

// NewService is the factory function for creating a new analysis service.
func NewService(
	fetcher PageFetcher,
	parser PageParser,
	checker LinkChecker,
	logger *slog.Logger,
) Service {
	return &serviceImpl{
		fetcher: fetcher,
		parser:  parser,
		checker: checker,
		logger:  logger,
	}
}

// AnalyzePage orchestrates the entire page analysis workflow.
func (s *serviceImpl) AnalyzePage(ctx context.Context, rawURL string) (*domain.AnalysisResult, *domain.AppError) {
	s.logger.InfoContext(ctx, "Starting page analysis", "url", rawURL)

	// 1. Validation & Parsing
	baseURL, appErr := s.normalizeURL(rawURL)
	if appErr != nil {
		s.logger.WarnContext(ctx, "Invalid URL provided", "url", rawURL, "error", appErr.InternalError())
		return nil, appErr
	}

	// 2. Fetch the page
	pageBody, appErr := s.fetcher.Fetch(ctx, baseURL.String())
	if appErr != nil {
		s.logger.ErrorContext(ctx, "Failed to fetch page", "url", baseURL.String(), "error", appErr)
		return nil, appErr
	}
	defer pageBody.Close()

	// 3. Parse the content
	s.logger.DebugContext(ctx, "Parsing page content", "url", baseURL.String())
	result, allLinks, err := s.parser.Parse(ctx, pageBody, baseURL)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to parse page content", "url", baseURL.String(), "error", err)
		return nil, domain.NewInternalError(constants.ErrURLParse, err)

	}

	// 4. Check links concurrently
	s.logger.DebugContext(ctx, "Checking links concurrently", "url", baseURL.String(), "link_count", len(allLinks))
	inaccessibleCount := s.checker.Check(ctx, allLinks)
	result.Links.Inaccessible = inaccessibleCount

	s.logger.InfoContext(ctx, "Page analysis complete", "url", baseURL.String(), "title", result.Title)
	return result, nil
}

// Helper function to ensure the URL is valid
func (s *serviceImpl) normalizeURL(rawURL string) (*url.URL, *domain.AppError) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, domain.NewAppErrorUser(http.StatusBadRequest, constants.ErrURLEmpty)
	}

	// scheme is missing, return error
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https") {
		return nil, domain.NewAppErrorUser(http.StatusBadRequest, constants.ErrURLSchemeMissing)
	}

	// Use net/url to parse
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, domain.NewAppError(http.StatusBadRequest, constants.ErrURLInvalidFormat, err)
	}

	if parsedURL.Host == "" {
		return nil, domain.NewAppErrorUser(http.StatusBadRequest, constants.ErrURLHostMissing)
	}

	return parsedURL, nil
}

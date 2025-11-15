package parser

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"webpage-analyzer-service/internal/analysis"
	"webpage-analyzer-service/internal/domain"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// htmlParser is the concrete implementation of the analysis.PageParser interface.
type htmlParser struct {
	logger *slog.Logger
}

var (
	// passwordHeuristics defines common 'name' or 'id' attributes for login fields.
	passwordHeuristics = []string{"password", "pass", "pwd", "user_pass", "user_password"}
)

// containsFold is a case-insensitive helper to check if a slice contains a string.
func containsFold(slice []string, val string) bool {
	val = strings.TrimSpace(val)
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}

// NewHTMLParser creates a new instance of htmlParser.
func NewHTMLParser(logger *slog.Logger) analysis.PageParser {
	return &htmlParser{
		logger: logger,
	}
}

// Parse implements the analysis.PageParser interface.
func (p *htmlParser) Parse(ctx context.Context, r io.Reader, baseURL *url.URL) (*domain.AnalysisResult, []string, error) {

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to create goquery document", "error", err)
		return nil, nil, err
	}

	// 1. Get HTML Version (by inspecting doctype)
	htmlVersion := p.findHTMLVersion(doc.Nodes[0])

	// 2. Get Title
	title := doc.Find("title").First().Text()

	// 3. Count Headings
	headings := make(map[string]int)
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		level := goquery.NodeName(s)
		headings[level]++
	})

	// 4. Find Login Form
	hasLoginForm := p.findLoginForm(doc)
	p.logger.InfoContext(ctx, "Login form detection status :", "found", hasLoginForm)

	// 5. Find all links and categorize them
	links := []string{}
	internalCount := 0
	externalCount := 0

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || strings.TrimSpace(href) == "" || strings.HasPrefix(href, "#") {
			return // Skip empty or anchor-only links
		}

		// Resolve the link relative to the base URL
		resolvedURL, err := baseURL.Parse(href)
		if err != nil {
			p.logger.WarnContext(ctx, "Failed to parse link", "href", href, "base_url", baseURL.String())
			return // Skip malformed links
		}

		// Add to the list for the link checker
		links = append(links, resolvedURL.String())

		// Categorize as internal or external
		if resolvedURL.Host == baseURL.Host {
			internalCount++
		} else {
			externalCount++
		}
	})

	result := &domain.AnalysisResult{
		HTMLVersion:  htmlVersion,
		Title:        strings.TrimSpace(title),
		Headings:     headings,
		HasLoginForm: hasLoginForm,
		Links: domain.LinkCounts{
			Internal: internalCount,
			External: externalCount,
			// Inaccessible will be filled in by the service layer
		},
	}

	return result, links, nil
}

// findLoginForm checks for inputs that indicate a login form.
// here main assumption is login attributes can exist without form element.
func (p *htmlParser) findLoginForm(doc *goquery.Document) bool {
	var found bool
	// Find all inputs and stop on the first match.
	doc.Find("input").EachWithBreak(func(j int, input *goquery.Selection) bool {

		// Check 1: type="password"
		if inputType, exists := input.Attr("type"); exists {
			if strings.EqualFold(strings.TrimSpace(inputType), "password") {
				found = true
				return false // Match found
			}
		}

		// Check 2: autocomplete="current-password"
		if autocomplete, exists := input.Attr("autocomplete"); exists {
			if strings.EqualFold(strings.TrimSpace(autocomplete), "current-password") {
				found = true
				return false // Match found
			}
		}

		// Check 3: 'name' attribute heuristic
		if name, exists := input.Attr("name"); exists {
			if containsFold(passwordHeuristics, name) {
				found = true
				return false // Match found
			}
		}

		// Check 4: 'id' attribute heuristic
		if id, exists := input.Attr("id"); exists {
			if containsFold(passwordHeuristics, id) {
				found = true
				return false // Match found
			}
		}

		return true // Continue iterating
	})

	return found
}

// findHTMLVersion analyzes the doctype to determine the HTML version.
func (p *htmlParser) findHTMLVersion(node *html.Node) string {
	if node == nil {
		return "Unknown"
	}

	// Recurse to find the first DoctypeNode
	var f func(*html.Node) *html.Node
	f = func(n *html.Node) *html.Node {
		if n.Type == html.DoctypeNode {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := f(c); result != nil {
				return result
			}
		}
		return nil
	}

	doctypeNode := f(node)
	if doctypeNode == nil {
		return "HTML (No Doctype)"
	}

	// Simple check for HTML5
	if doctypeNode.Data == "html" && len(doctypeNode.Attr) == 0 {
		return "HTML 5"
	}

	// Check for older versions
	for _, attr := range doctypeNode.Attr {
		if attr.Key == "public" {
			if strings.Contains(attr.Val, "XHTML 1.0") {
				return "XHTML 1.0"
			}
			if strings.Contains(attr.Val, "HTML 4.01") {
				return "HTML 4.01"
			}
		}
	}

	return "Unknown"
}

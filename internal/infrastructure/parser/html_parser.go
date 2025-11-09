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
	htmlVersion := p.findHTMLVersion(doc.First().Nodes[0])

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
		// This correctly handles links like "/about" or "../contact"
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

// findLoginForm checks for the presence of a form containing a password input.
func (p *htmlParser) findLoginForm(doc *goquery.Document) bool {
	found := false
	// Find all forms on the page
	doc.Find("form").EachWithBreak(func(i int, form *goquery.Selection) bool {
		// Check if this form contains an input of type 'password'
		if form.Find("input[type='password']").Length() > 0 {
			found = true
			return false
		}
		return true
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

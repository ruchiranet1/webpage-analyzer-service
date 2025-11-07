package analyzer

// Next stage possible improvements :
// - Add content analysis
// - Add `faviconURL` extraction
type AnalysisResult struct {
	HTMLVersion string         `json:"htmlVersion"`
	PageTitle   string         `json:"pageTitle"`
	Headings    map[string]int `json:"headings"`
	Links       LinkStats      `json:"links"`
	// True if password input in form
	HasLoginForm bool `json:"hasLoginForm"`
}

type LinkStats struct {
	// Links to same domain
	Internal int `json:"internal"`
	// Links to different domain
	External int `json:"external"`
	// Links that failed to resolve
	Inaccessible int `json:"inaccessible"`
}

package domain

// AnalysisRequest defines the payload for an analysis request
// {"requestId":"demo1","email":"webUser1@gmail.com","url":"https://httpbin.org/html"}
type AnalysisRequest struct {
	RequestID string `json:"requestId"`
	Email     string `json:"email"`
	URL       string `json:"url"`
}

// Primary object returned to the user.
type AnalysisResult struct {
	HTMLVersion  string         `json:"html_version"`
	Title        string         `json:"title"`
	Headings     map[string]int `json:"headings"`
	Links        LinkCounts     `json:"links"`
	HasLoginForm bool           `json:"has_login_form"`
}

// LinkCounts provides a detailed breakdown of link types found on the page.
type LinkCounts struct {
	Internal     int `json:"internal"`
	External     int `json:"external"`
	Inaccessible int `json:"inaccessible"`
}

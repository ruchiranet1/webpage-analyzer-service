// internal/analyzer/types_test.go
package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalysisResult_JSON(t *testing.T) {
	result := AnalysisResult{
		HTMLVersion:  "HTML5",
		PageTitle:    "Test Page",
		Headings:     map[string]int{"h1": 2, "h2": 3},
		Links:        LinkStats{Internal: 5, External: 2, Inaccessible: 1},
		HasLoginForm: true,
	}

	data, err := json.Marshal(result)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "Test Page")
	assert.Contains(t, string(data), "h2")
	assert.Contains(t, string(data), "inaccessible")
}

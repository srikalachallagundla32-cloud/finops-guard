package costengine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/finops-guard/pkg/costengine"
)

func TestCalculateLLMLoopCost(t *testing.T) {
	// Setup temporary test pricing JSON
	mockJSON := `{
		"project_name": "FinOps-Guard-Test",
		"currency": "USD",
		"region_default": "us-east-1",
		"pricing_data": {
			"openai_api": {
				"gpt_4o": {
					"cost_per_million_input_tokens": 2.50,
					"cost_per_million_output_tokens": 10.00
				},
				"gpt_4o_mini": {
					"cost_per_million_input_tokens": 0.15,
					"cost_per_million_output_tokens": 0.60
				}
			},
			"anthropic_api": {
				"claude_3_5_sonnet": {
					"cost_per_million_input_tokens": 3.00,
					"cost_per_million_output_tokens": 15.00
				}
			}
		}
	}`

	tmpDir := t.TempDir()
	pricingPath := filepath.Join(tmpDir, "pricing.json")
	if err := os.WriteFile(pricingPath, []byte(mockJSON), 0644); err != nil {
		t.Fatalf("failed to write mock pricing file: %v", err)
	}

	catalog, err := costengine.LoadPricingCatalog(pricingPath)
	if err != nil {
		t.Fatalf("expected no error loading pricing catalog, got: %v", err)
	}

	// Table-driven test cases
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		iterations   int
		expectedCost float64
	}{
		{
			name:         "GPT-4o 1000 iterations",
			model:        "gpt-4o",
			inputTokens:  1000,
			outputTokens: 500,
			iterations:   1000,
			expectedCost: 7.50, // ((1000/1e6)*2.50 + (500/1e6)*10.00) * 1000 = (0.0025 + 0.005) * 1000 = 7.50
		},
		{
			name:         "GPT-4o-Mini 1000 iterations",
			model:        "gpt-4o-mini",
			inputTokens:  1000,
			outputTokens: 500,
			iterations:   1000,
			expectedCost: 0.45, // ((1000/1e6)*0.15 + (500/1e6)*0.60) * 1000 = (0.00015 + 0.00030) * 1000 = 0.45
		},
		{
			name:         "Claude 3.5 Sonnet 500 iterations",
			model:        "claude-3-5-sonnet",
			inputTokens:  2000,
			outputTokens: 1000,
			iterations:   500,
			expectedCost: 10.50, // ((2000/1e6)*3.00 + (1000/1e6)*15.00) * 500 = (0.006 + 0.015) * 500 = 10.50
		},
		{
			name:         "Unknown model fallback to GPT-4o",
			model:        "unknown-llm",
			inputTokens:  1000,
			outputTokens: 500,
			iterations:   100,
			expectedCost: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := catalog.CalculateLLMLoopCost(tt.model, tt.inputTokens, tt.outputTokens, tt.iterations)
			if got != tt.expectedCost {
				t.Errorf("CalculateLLMLoopCost() = %v; want %v", got, tt.expectedCost)
			}
		})
	}
}

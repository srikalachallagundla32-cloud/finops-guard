package costengine

import (
	"encoding/json"
	"fmt"
	"os"
)

type PricingCatalog struct {
	ProjectName   string        `json:"project_name"`
	Currency      string        `json:"currency"`
	RegionDefault string        `json:"region_default"`
	PricingData   PricingMatrix `json:"pricing_data"`
}

type PricingMatrix struct {
	AmazonAthena   AthenaPricing    `json:"amazon_athena"`
	OpenAIAPI      OpenAIPricing    `json:"openai_api"`
	AnthropicAPI   AnthropicPricing `json:"anthropic_api"`
	AmazonDynamoDB DynamoDBPricing  `json:"amazon_dynamodb"`
}

type AthenaPricing struct {
	CostPerTBScanned     float64 `json:"cost_per_tb_scanned"`
	MinimumBytesPerQuery float64 `json:"minimum_bytes_per_query"`
}

type TokenCost struct {
	InputPerMillion  float64 `json:"cost_per_million_input_tokens"`
	OutputPerMillion float64 `json:"cost_per_million_output_tokens"`
}

type OpenAIPricing struct {
	GPT4o     TokenCost `json:"gpt_4o"`
	GPT4oMini TokenCost `json:"gpt_4o_mini"`
}

type AnthropicPricing struct {
	Claude35Sonnet TokenCost `json:"claude_3_5_sonnet"`
}

type DynamoDBPricing struct {
	CostPerMillionWRU float64 `json:"cost_per_million_write_request_units"`
	CostPerMillionRRU float64 `json:"cost_per_million_read_request_units"`
}

func LoadPricingCatalog(filePath string) (*PricingCatalog, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pricing file: %w", err)
	}

	var catalog PricingCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pricing json: %w", err)
	}

	return &catalog, nil
}

func (c *PricingCatalog) CalculateLLMLoopCost(model string, inputTokens, outputTokens, estimatedIterations int) float64 {
	var pricing TokenCost

	switch model {
	case "gpt-4o":
		pricing = c.PricingData.OpenAIAPI.GPT4o
	case "gpt-4o-mini":
		pricing = c.PricingData.OpenAIAPI.GPT4oMini
	case "claude-3-5-sonnet":
		pricing = c.PricingData.AnthropicAPI.Claude35Sonnet
	default:
		pricing = c.PricingData.OpenAIAPI.GPT4o
	}

	costPerCall := ((float64(inputTokens) / 1e6) * pricing.InputPerMillion) +
		((float64(outputTokens) / 1e6) * pricing.OutputPerMillion)

	return costPerCall * float64(estimatedIterations)
}

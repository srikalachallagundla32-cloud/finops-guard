package costengine

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// RoundCurrency rounds a dollar amount to 4 decimal places so cost figures
// stay clean across calculations, tests, and UI output. 4 decimals (not 2)
// because sub-cent per-call/per-unit costs (e.g. a single DynamoDB write
// request unit) would otherwise round to $0.00 and lose all signal.
func RoundCurrency(cost float64) float64 {
	return math.Round(cost*10000) / 10000
}

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

	return RoundCurrency(costPerCall * float64(estimatedIterations))
}

// CalculateAthenaQueryCost estimates the cost of one Athena query at the
// billing-minimum bytes scanned.
func (c *PricingCatalog) CalculateAthenaQueryCost() float64 {
	tbScanned := c.PricingData.AmazonAthena.MinimumBytesPerQuery / 1e12
	return RoundCurrency(tbScanned * c.PricingData.AmazonAthena.CostPerTBScanned)
}

// CalculateDynamoDBWriteCost estimates the cost of a single DynamoDB write
// request unit.
func (c *PricingCatalog) CalculateDynamoDBWriteCost() float64 {
	return RoundCurrency(c.PricingData.AmazonDynamoDB.CostPerMillionWRU / 1e6)
}

const (
	defaultCallInputTokens  = 1000
	defaultCallOutputTokens = 500
)

// Flat per-call cost estimates for providers not modelled in pricing.json.
// These are conservative published-rate approximations (USD per call).
const (
	CostBedrockClaudeSonnet = 0.015  // AWS Bedrock Claude 3.5 Sonnet estimate
	CostVertexGeminiPro     = 0.0025 // GCP Vertex AI Gemini Pro estimate
	CostPineconeQuery       = 0.001  // Pinecone vector query estimate
	CostRetokenization      = 0.015  // Re-sending full growing chat history (FG-010)
	CostConcurrencyBlast    = 0.02   // Unthrottled Promise.all fan-out in a loop (FG-011)
)

// EstimateCallCost estimates the cost of a single call to targetAPI
// ("openai", "anthropic", "athena", "dynamodb", "bedrock", "vertex",
// "pinecone"), using default assumed token counts for LLM calls,
// billing-minimum units for cloud APIs, and flat estimates for the rest.
func (c *PricingCatalog) EstimateCallCost(targetAPI string) float64 {
	switch targetAPI {
	case "openai":
		return c.CalculateLLMLoopCost("gpt-4o", defaultCallInputTokens, defaultCallOutputTokens, 1)
	case "anthropic":
		return c.CalculateLLMLoopCost("claude-3-5-sonnet", defaultCallInputTokens, defaultCallOutputTokens, 1)
	case "athena":
		return c.CalculateAthenaQueryCost()
	case "dynamodb":
		return c.CalculateDynamoDBWriteCost()
	case "bedrock":
		return RoundCurrency(CostBedrockClaudeSonnet)
	case "vertex":
		return RoundCurrency(CostVertexGeminiPro)
	case "pinecone":
		return RoundCurrency(CostPineconeQuery)
	case "retokenization":
		return RoundCurrency(CostRetokenization)
	case "concurrency":
		return RoundCurrency(CostConcurrencyBlast)
	default:
		return 0
	}
}

package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Issue represents a detected loop-bound API call in a scanned file.
type Issue struct {
	ID          string  `json:"id"`
	RuleName    string  `json:"rule_name"`
	FilePath    string  `json:"file_path"`
	LineNumber  int     `json:"line_number"`
	CodeSnippet string  `json:"code_snippet"`
	Severity    string  `json:"severity"`
	TargetAPI   string  `json:"target_api"`
	EstCostRisk float64 `json:"est_cost_risk"`
}

// DetectionRule matches a target API call pattern.
type DetectionRule struct {
	ID        string
	Name      string
	Severity  string
	TargetAPI string
	Pattern   *regexp.Regexp
}

// GetDefaultRules returns extended regex rules for LLM, AWS Athena, and DynamoDB.
func GetDefaultRules() []DetectionRule {
	return []DetectionRule{
		{
			ID:        "FG-001",
			Name:      "OpenAI API Call in Loop",
			Severity:  "HIGH",
			TargetAPI: "openai",
			Pattern:   regexp.MustCompile(`(openai\.(ChatCompletion|chat\.completions|client\.chat\.completions)\.create|client\.messages\.create)`),
		},
		{
			ID:        "FG-002",
			Name:      "Anthropic API Call in Loop",
			Severity:  "HIGH",
			TargetAPI: "anthropic",
			Pattern:   regexp.MustCompile(`(anthropic\.(messages|completions)\.create|client\.messages\.create)`),
		},
		{
			ID:        "FG-003",
			Name:      "AWS Athena Query Execution in Loop",
			Severity:  "CRITICAL",
			TargetAPI: "athena",
			Pattern:   regexp.MustCompile(`(athena\.start_query_execution|boto3\.client\('athena'\)|AthenaClient\.startQueryExecution)`),
		},
		{
			ID:        "FG-004",
			Name:      "DynamoDB Write Operations in Async Loop",
			Severity:  "HIGH",
			TargetAPI: "dynamodb",
			Pattern:   regexp.MustCompile(`(dynamodb\.(put_item|batch_write_item)|docClient\.send\(new PutItemCommand)`),
		},
	}
}

var (
	loopKeywordPattern = regexp.MustCompile(`^\s*(for|while)\b`)
	loopMethodPattern  = regexp.MustCompile(`\.(forEach|map)\s*\(`)
)

// loopScope tracks one open loop body, either by brace nesting (JS/TS)
// or by indentation depth (Python).
type loopScope struct {
	braceTracked bool
	indent       int
	braceBalance int
}

func indentOf(line string) int {
	count := 0
	for _, r := range line {
		switch r {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			return count
		}
	}
	return count
}

// ScanFile reads a Python/TypeScript file line by line, tracks loop scopes
// (for/while blocks and .forEach/.map callbacks), and flags any line inside
// an active loop scope that matches one of the given detection rules.
func ScanFile(filePath string, rules []DetectionRule) ([]Issue, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var issues []Issue
	var scopes []loopScope

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		trimmed := strings.TrimSpace(rawLine)

		if trimmed != "" {
			currentIndent := indentOf(rawLine)
			for len(scopes) > 0 {
				top := scopes[len(scopes)-1]
				if !top.braceTracked && currentIndent <= top.indent {
					scopes = scopes[:len(scopes)-1]
					continue
				}
				break
			}
		}

		isLoopStart := loopKeywordPattern.MatchString(trimmed) || loopMethodPattern.MatchString(trimmed)
		if isLoopStart {
			scopes = append(scopes, loopScope{
				braceTracked: strings.Contains(rawLine, "{"),
				indent:       indentOf(rawLine),
			})
		}

		opens := strings.Count(rawLine, "{")
		closes := strings.Count(rawLine, "}")
		if opens > 0 || closes > 0 {
			for i := range scopes {
				if scopes[i].braceTracked {
					scopes[i].braceBalance += opens - closes
				}
			}
			for len(scopes) > 0 {
				top := scopes[len(scopes)-1]
				if top.braceTracked && top.braceBalance <= 0 {
					scopes = scopes[:len(scopes)-1]
					continue
				}
				break
			}
		}

		if len(scopes) == 0 {
			continue
		}

		for _, rule := range rules {
			if rule.Pattern.MatchString(trimmed) {
				issues = append(issues, Issue{
					ID:          rule.ID,
					RuleName:    rule.Name,
					FilePath:    filePath,
					LineNumber:  lineNumber,
					CodeSnippet: trimmed,
					Severity:    rule.Severity,
					TargetAPI:   rule.TargetAPI,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return issues, nil
}

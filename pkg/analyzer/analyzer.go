package analyzer

import (
	"bufio"
	"fmt"
	"io"
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
		{
			ID:        "FG-005",
			Name:      "AWS Bedrock API Call in Loop",
			Severity:  "CRITICAL",
			TargetAPI: "bedrock",
			// Python boto3 (bedrock-runtime.invoke_model / .converse) and the
			// AWS SDK for JS command classes.
			Pattern: regexp.MustCompile(`(bedrock[\w.\-]*\.(invoke_model|converse)|\binvoke_model\s*\(|new\s+(InvokeModel|Converse)Command)`),
		},
		{
			ID:        "FG-006",
			Name:      "GCP Vertex AI Call in Loop",
			Severity:  "HIGH",
			TargetAPI: "vertex",
			// Python generate_content(...) and JS/TS generateContent(...).
			Pattern: regexp.MustCompile(`(generate_content|generateContent)\s*\(`),
		},
		{
			ID:        "FG-007",
			Name:      "Vector DB Query in Loop",
			Severity:  "HIGH",
			TargetAPI: "pinecone",
			// Pinecone / vector index query & upsert (Python and JS/TS).
			Pattern: regexp.MustCompile(`\b(index|pinecone\w*)\.(query|upsert)\s*\(`),
		},
		{
			ID:        "FG-010",
			Name:      "Full History Re-tokenization in Loop",
			Severity:  "HIGH",
			TargetAPI: "retokenization",
			// Re-sending the whole growing history: .create(... messages=messages ...)
			// (Python kwarg) or .create({ messages: messages }) (JS/TS). Single-line
			// on purpose — the engine matches per line inside an already-detected
			// loop scope, so the proposed multi-line (?s)(for|while).* patterns
			// (and the backref/lookahead FG-008/FG-009) don't fit and are deferred.
			Pattern: regexp.MustCompile(`\.create\(.*\bmessages\s*[:=]\s*messages\b`),
		},
		{
			ID:        "FG-011",
			Name:      "Unthrottled Parallel Async Blast",
			Severity:  "HIGH",
			TargetAPI: "concurrency",
			// A Promise.all fan-out sitting inside a loop scope (or a .map/.forEach
			// callback) — simultaneous requests risk rate-limit bursts.
			Pattern: regexp.MustCompile(`Promise\.all\s*\(`),
		},
		{
			ID:        "FG-012",
			Name:      "Hardcoded Secret in Loop",
			Severity:  "CRITICAL",
			TargetAPI: "secret",
			// OpenAI project key or AWS access key id literal inside a loop scope.
			Pattern: regexp.MustCompile(`(sk-proj-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16})`),
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
	return scan(file, filePath, rules)
}

// ScanCodeSnippet scans in-memory source with the default rule set. filePath
// is used only for issue reporting and the .py/.ts language heuristics.
func ScanCodeSnippet(code, filePath string) []Issue {
	issues, _ := scan(strings.NewReader(code), filePath, GetDefaultRules())
	return issues
}

// scan is the shared line-by-line, loop-scope-tracking engine behind both
// ScanFile and ScanCodeSnippet.
func scan(r io.Reader, filePath string, rules []DetectionRule) ([]Issue, error) {
	var issues []Issue
	var scopes []loopScope

	scanner := bufio.NewScanner(r)
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

// remediationComment builds the advisory comment inserted above a flagged
// line. This intentionally never rewrites the surrounding loop logic —
// ScanFile's regex/indentation heuristics aren't a real parser, so anything
// beyond a plain comment risks silently corrupting the developer's file.
func remediationComment(issue Issue) string {
	prefix := "//"
	if strings.HasSuffix(issue.FilePath, ".py") {
		prefix = "#"
	}
	return fmt.Sprintf("%s FINOPS-GUARD [%s]: batch this %s call outside the loop before merging.", prefix, issue.ID, issue.TargetAPI)
}

func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

// fixPlan loads the target file and computes where the remediation comment
// would be inserted, without writing anything back.
func fixPlan(issue Issue) (lines []string, insertIdx int, comment string, err error) {
	data, err := os.ReadFile(issue.FilePath)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to read file %s: %w", issue.FilePath, err)
	}

	lines = strings.Split(string(data), "\n")
	insertIdx = issue.LineNumber - 1
	if insertIdx < 0 || insertIdx >= len(lines) {
		return nil, 0, "", fmt.Errorf("line %d out of range for %s", issue.LineNumber, issue.FilePath)
	}

	comment = leadingWhitespace(lines[insertIdx]) + remediationComment(issue)
	return lines, insertIdx, comment, nil
}

// PreviewFix returns a short before/after snippet (two lines of context on
// each side of the flagged line) without modifying the file on disk.
func PreviewFix(issue Issue) (before string, after string, err error) {
	lines, insertIdx, comment, err := fixPlan(issue)
	if err != nil {
		return "", "", err
	}

	start := insertIdx - 2
	if start < 0 {
		start = 0
	}
	end := insertIdx + 3
	if end > len(lines) {
		end = len(lines)
	}

	before = strings.Join(lines[start:end], "\n")

	afterLines := make([]string, 0, end-start+1)
	afterLines = append(afterLines, lines[start:insertIdx]...)
	afterLines = append(afterLines, comment)
	afterLines = append(afterLines, lines[insertIdx:end]...)
	after = strings.Join(afterLines, "\n")

	return before, after, nil
}

// SuggestionBlock returns the replacement text for a committable GitHub review
// suggestion anchored to the flagged line: the remediation comment inserted
// above, followed by the original line, both preserving indentation. This is
// the same conservative edit ApplyFix performs (a warning annotation, never a
// speculative refactor), packaged for a one-click "Commit suggestion".
func SuggestionBlock(issue Issue) (replacement string, err error) {
	lines, insertIdx, comment, err := fixPlan(issue)
	if err != nil {
		return "", err
	}
	return comment + "\n" + lines[insertIdx], nil
}

// ApplyFix inserts the FinOps-Guard remediation comment directly above the
// flagged line and writes the file back to disk, preserving its existing
// file mode. Callers must obtain developer confirmation before calling this
// — it performs no confirmation of its own.
func ApplyFix(issue Issue) error {
	lines, insertIdx, comment, err := fixPlan(issue)
	if err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if info, statErr := os.Stat(issue.FilePath); statErr == nil {
		mode = info.Mode()
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, comment)
	newLines = append(newLines, lines[insertIdx:]...)

	if err := os.WriteFile(issue.FilePath, []byte(strings.Join(newLines, "\n")), mode); err != nil {
		return fmt.Errorf("failed to write file %s: %w", issue.FilePath, err)
	}
	return nil
}

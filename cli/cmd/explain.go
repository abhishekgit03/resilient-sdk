package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
)

var explainLast string

var explainCmd = &cobra.Command{
	Use:   "explain <service>",
	Short: "AI-powered analysis of a service's failure patterns",
	Example: `  resilient explain openai
  resilient explain stripe --last 7d`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := args[0]

		if Cfg.GeminiAPIKey == "" {
			return fmt.Errorf("no Gemini API key — run: resilient init --dsn <dsn> --gemini-key <key>")
		}

		interval, err := toPostgresInterval(explainLast)
		if err != nil {
			return err
		}

		stats, err := fetchServiceStats(service, interval)
		if err != nil {
			return fmt.Errorf("fetch stats: %w", err)
		}

		if stats == "" {
			fmt.Printf("No data found for service %q in the last %s.\n", service, explainLast)
			return nil
		}

		fmt.Printf("Analysing %s (last %s)...\n\n", service, explainLast)

		explanation, err := callGemini(Cfg.GeminiAPIKey, service, stats)
		if err != nil {
			return fmt.Errorf("gemini: %w", err)
		}

		fmt.Println(explanation)
		return nil
	},
}

func init() {
	explainCmd.Flags().StringVar(&explainLast, "last", "7d", "Time window: 1h, 24h, 7d, 30d")
}

func fetchServiceStats(service, interval string) (string, error) {
	query := `
		SELECT
			fn,
			COUNT(*)                                                    AS total,
			COUNT(*) FILTER (WHERE status = 'failure')                  AS failures,
			ROUND(
				COUNT(*) FILTER (WHERE status = 'failure')::numeric
				/ NULLIF(COUNT(*), 0) * 100, 2
			)                                                           AS failure_pct,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms)  AS p95_ms,
			MAX(attempt)                                                AS max_attempts,
			MODE() WITHIN GROUP (ORDER BY error_type)                   AS top_error_type,
			EXTRACT(HOUR FROM ts AT TIME ZONE 'UTC')::int               AS peak_hour
		FROM resilient.events
		WHERE service = $1
		  AND ts >= now() - $2::interval
		GROUP BY fn, EXTRACT(HOUR FROM ts AT TIME ZONE 'UTC')
		ORDER BY failure_pct DESC NULLS LAST
		LIMIT 10
	`

	rows, err := DB.Query(context.Background(), query, service, interval)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var fn, topErrorType string
		var total, failures, maxAttempts, peakHour int
		var failurePct, p95 float64

		if err := rows.Scan(&fn, &total, &failures, &failurePct, &p95, &maxAttempts, &topErrorType, &peakHour); err != nil {
			return "", err
		}

		lines = append(lines, fmt.Sprintf(
			"function=%s total=%d failures=%d failure_pct=%.2f%% p95_ms=%.0f max_attempts=%d top_error=%s peak_hour_utc=%d",
			fn, total, failures, failurePct, p95, maxAttempts, topErrorType, peakHour,
		))
	}

	return strings.Join(lines, "\n"), nil
}

func callGemini(apiKey, service, stats string) (string, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("create client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.SetTemperature(0.3)

	prompt := buildPrompt(service, stats)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func buildPrompt(service, stats string) string {
	return fmt.Sprintf(`You are an expert backend reliability engineer analysing retry failure data from a production system.

Service: %s

Failure statistics (aggregated from retry event logs):
%s

Write a concise, actionable analysis (4-6 sentences) covering:
1. What the current failure rate is and whether it is concerning
2. What the dominant error type suggests about the root cause
3. What the p95 latency and max retry attempts tell you
4. One concrete recommendation to improve reliability (e.g. backoff tuning, circuit breaker, batching)

Be direct. No bullet points. Write in plain English as if explaining to the developer who owns this service.`, service, stats)
}

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	reportApp  string
	reportLast string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show failure summary across services",
	Example: `  resilient report
  resilient report --app openai --last 7d`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, err := toPostgresInterval(reportLast)
		if err != nil {
			return err
		}

		query := `
			SELECT
				service,
				fn,
				COUNT(*)                                            AS total_calls,
				COUNT(*) FILTER (WHERE status = 'failure')         AS failures,
				ROUND(
					COUNT(*) FILTER (WHERE status = 'failure')::numeric
					/ NULLIF(COUNT(*), 0) * 100, 2
				)                                                   AS failure_pct,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_ms
			FROM resilient.events
			WHERE ts >= now() - $1::interval
			  AND ($2::text IS NULL OR service = $2)
			GROUP BY service, fn
			ORDER BY failure_pct DESC NULLS LAST, total_calls DESC
		`

		rows, err := DB.Query(context.Background(), query, interval, nullableString(reportApp))
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Service", "Function", "Total", "Failures", "Failure %", "p95 (ms)"})
		table.SetBorder(false)
		table.SetColumnSeparator("  ")

		found := false
		for rows.Next() {
			found = true
			var service, fn string
			var total, failures int
			var failurePct float64
			var p95 float64

			if err := rows.Scan(&service, &fn, &total, &failures, &failurePct, &p95); err != nil {
				return err
			}
			table.Append([]string{
				service,
				fn,
				fmt.Sprintf("%d", total),
				fmt.Sprintf("%d", failures),
				fmt.Sprintf("%.2f%%", failurePct),
				fmt.Sprintf("%.0f", p95),
			})
		}

		if !found {
			fmt.Println("No events found for the given filters.")
			return nil
		}

		fmt.Printf("\nRetry Report — last %s\n\n", reportLast)
		table.Render()
		return nil
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportApp, "app", "", "Filter by service name")
	reportCmd.Flags().StringVar(&reportLast, "last", "24h", "Time window: 1h, 24h, 7d, 30d")
}

func toPostgresInterval(last string) (string, error) {
	switch last {
	case "1h":
		return "1 hour", nil
	case "24h":
		return "24 hours", nil
	case "7d":
		return "7 days", nil
	case "30d":
		return "30 days", nil
	default:
		return "", fmt.Errorf("unsupported --last value %q: use 1h, 24h, 7d, or 30d", last)
	}
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var topLast string

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show the worst-performing services in a time window",
	Example: `  resilient top
  resilient top --last 24h`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, err := toPostgresInterval(topLast)
		if err != nil {
			return err
		}

		query := `
			SELECT
				service,
				fn,
				COUNT(*)                                            AS total,
				COUNT(*) FILTER (WHERE status = 'failure')         AS failures,
				ROUND(
					COUNT(*) FILTER (WHERE status = 'failure')::numeric
					/ NULLIF(COUNT(*), 0) * 100, 2
				)                                                   AS failure_pct,
				MAX(attempt)                                        AS max_attempts,
				ROUND(AVG(duration_ms))                             AS avg_ms
			FROM resilient.events
			WHERE ts >= now() - $1::interval
			GROUP BY service, fn
			HAVING COUNT(*) FILTER (WHERE status = 'failure') > 0
			ORDER BY failure_pct DESC, failures DESC
			LIMIT 10
		`

		rows, err := DB.Query(context.Background(), query, interval)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Service", "Function", "Total", "Failures", "Failure %", "Max Attempts", "Avg ms"})
		table.SetBorder(false)
		table.SetColumnSeparator("  ")

		found := false
		for rows.Next() {
			found = true
			var service, fn string
			var total, failures, maxAttempts int
			var failurePct, avgMs float64
			if err := rows.Scan(&service, &fn, &total, &failures, &failurePct, &maxAttempts, &avgMs); err != nil {
				return err
			}
			table.Append([]string{
				service, fn,
				fmt.Sprintf("%d", total),
				fmt.Sprintf("%d", failures),
				fmt.Sprintf("%.2f%%", failurePct),
				fmt.Sprintf("%d", maxAttempts),
				fmt.Sprintf("%.0f", avgMs),
			})
		}

		if !found {
			fmt.Printf("No failures in the last %s.\n", topLast)
			return nil
		}

		fmt.Printf("\nTop offenders: last %s\n\n", topLast)
		table.Render()
		return nil
	},
}

func init() {
	topCmd.Flags().StringVar(&topLast, "last", "1h", "Time window: 1h, 24h, 7d, 30d")
}

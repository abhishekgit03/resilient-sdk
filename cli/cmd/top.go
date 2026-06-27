package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show the worst-performing services in the last hour",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			WHERE ts >= now() - interval '1 hour'
			GROUP BY service, fn
			HAVING COUNT(*) FILTER (WHERE status = 'failure') > 0
			ORDER BY failure_pct DESC, failures DESC
			LIMIT 10
		`

		rows, err := DB.Query(context.Background(), query)
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
			fmt.Println("No failures in the last hour.")
			return nil
		}

		fmt.Println("\nTop offenders: last 1 hour\n")
		table.Render()
		return nil
	},
}

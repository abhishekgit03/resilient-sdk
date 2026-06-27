package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var anomaliesCmd = &cobra.Command{
	Use:   "anomalies",
	Short: "Show services whose failure rate spiked compared to yesterday",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Compare failure rate in the last 24h vs the 24h before that.
		// A spike is defined as: today's rate is at least 2x yesterday's rate
		// AND today's rate is above 1% (to avoid noise from low-volume services).
		query := `
			WITH periods AS (
				SELECT
					service,
					fn,
					ROUND(
						COUNT(*) FILTER (WHERE status = 'failure' AND ts >= now() - interval '24 hours')::numeric
						/ NULLIF(COUNT(*) FILTER (WHERE ts >= now() - interval '24 hours'), 0) * 100, 2
					) AS today_pct,
					ROUND(
						COUNT(*) FILTER (WHERE status = 'failure'
							AND ts >= now() - interval '48 hours'
							AND ts <  now() - interval '24 hours')::numeric
						/ NULLIF(COUNT(*) FILTER (WHERE ts >= now() - interval '48 hours'
							AND ts < now() - interval '24 hours'), 0) * 100, 2
					) AS yesterday_pct
				FROM resilient.events
				WHERE ts >= now() - interval '48 hours'
				GROUP BY service, fn
			)
			SELECT service, fn, today_pct, yesterday_pct,
				ROUND(today_pct - yesterday_pct, 2) AS delta
			FROM periods
			WHERE today_pct > 1
			  AND today_pct >= yesterday_pct * 2
			ORDER BY delta DESC
		`

		rows, err := DB.Query(context.Background(), query)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Service", "Function", "Today %", "Yesterday %", "Delta"})
		table.SetBorder(false)
		table.SetColumnSeparator("  ")

		found := false
		for rows.Next() {
			found = true
			var service, fn string
			var todayPct, yesterdayPct, delta float64
			if err := rows.Scan(&service, &fn, &todayPct, &yesterdayPct, &delta); err != nil {
				return err
			}
			table.Append([]string{
				service, fn,
				fmt.Sprintf("%.2f%%", todayPct),
				fmt.Sprintf("%.2f%%", yesterdayPct),
				fmt.Sprintf("+%.2f%%", delta),
			})
		}

		if !found {
			fmt.Println("No anomalies detected in the last 24h.")
			return nil
		}

		fmt.Println("\nAnomalies: failure rate spikes vs yesterday\n")
		table.Render()
		return nil
	},
}

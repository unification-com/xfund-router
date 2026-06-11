package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"go-ooo/database/models"
	"go-ooo/logger"
	"go-ooo/report"

	"github.com/spf13/cobra"
)

var (
	reportDays       int
	reportXfundPrice float64
	reportTop        int
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Operator report from the request DB: P&L, per-consumer/pair breakdown and failures",
	Long: "Reads the request history (sqlite by default, or a Postgres dump via the --db-* flags) " +
		"and prints an overall P&L summary plus per-consumer, per-pair and failure breakdowns. " +
		"Read-only: it does not migrate or modify the database.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := baseConfig()
		cfg.Log.Level = "error" // keep the report output clean
		logger.SetLogLevel(cfg.Log.Level)

		db := openDb(cfg)

		// The report is read-only and never migrates, so a database that has never run the oracle
		// (or a wrong --db path) has no tables. Report that clearly rather than failing on the query.
		if !db.Migrator().HasTable(&models.DataRequests{}) {
			fmt.Println("No request history found: this database has no 'data_requests' table.")
			fmt.Println("Point the --db-* flags at the oracle's database, or a dump of it.")
			return
		}

		var since time.Time
		if reportDays > 0 {
			since = time.Now().AddDate(0, 0, -reportDays)
		}

		requests, err := db.GetRequestsForReport(since)
		if err != nil {
			logger.Fatal("cmd", "report", "GetRequestsForReport", err.Error())
		}
		failed, err := db.GetAllFailedFulfilments()
		if err != nil {
			logger.Fatal("cmd", "report", "GetAllFailedFulfilments", err.Error())
		}

		printReport(report.Build(requests, failed, reportXfundPrice), reportDays, reportTop)
	},
}

func init() {
	reportCmd.Flags().IntVar(&reportDays, "days", 0, "only include requests from the last N days (0 = all history)")
	reportCmd.Flags().Float64Var(&reportXfundPrice, "xfund-price-eth", 0, "ETH per xFUND, to value fees + P&L in ETH (0 = skip the ETH figures)")
	reportCmd.Flags().IntVar(&reportTop, "top", 20, "show at most N rows in each breakdown (0 = all)")
	rootCmd.AddCommand(reportCmd)
}

func printReport(r report.Report, days, top int) {
	window := "all history"
	if days > 0 {
		window = fmt.Sprintf("last %d days", days)
	}
	o := r.Overall

	fmt.Printf("\nOoO provider report (%s)\n", window)
	fmt.Println("============================================")
	fmt.Printf("Requests analysed   : %d\n", o.TotalRequests)
	fmt.Printf("  successful        : %d\n", o.Successful)
	fmt.Printf("  fulfilment-failed : %d\n", o.FulfilmentFailed)
	fmt.Printf("  pending/in-flight : %d\n", o.Pending)
	fmt.Printf("Success rate        : %s\n", pct(o.SuccessRatePct, o.Successful+o.FulfilmentFailed))
	fmt.Printf("Reverted attempts   : %d\n", o.RevertedAttempts)
	fmt.Println("--------------------------------------------")
	fmt.Printf("Fees earned         : %.9f xFUND\n", o.FeesEarnedXfund)
	fmt.Printf("Gas cost            : %.9f ETH\n", o.GasCostEth)
	if r.XfundPriceEth > 0 {
		fmt.Printf("xFUND price         : %.9f ETH\n", r.XfundPriceEth)
		fmt.Printf("Fees earned         : %.9f ETH\n", o.FeesEarnedEth)
		fmt.Printf("Profit / loss       : %.9f ETH\n", o.ProfitLossEth)
	} else {
		fmt.Println("Profit / loss       : pass --xfund-price-eth to value fees + P&L in ETH")
	}

	printGroups("By consumer", "consumer", r.ByConsumer, top, r.XfundPriceEth)
	printGroups("By pair", "pair", r.ByPair, top, r.XfundPriceEth)
	printFailures(r.Failures, top)
	fmt.Println()
}

func printGroups(title, keyHeader string, groups []report.Group, top int, xfundPriceEth float64) {
	fmt.Printf("\n%s (%d)\n", title, len(groups))
	if len(groups) == 0 {
		fmt.Println("  (none)")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	header := fmt.Sprintf("%s\treqs\tok\tfailed\tfees(xFUND)\tgas(ETH)", keyHeader)
	if xfundPriceEth > 0 {
		header += "\tP/L(ETH)"
	}
	fmt.Fprintln(w, header)
	for i, g := range groups {
		if top > 0 && i >= top {
			fmt.Fprintf(w, "... and %d more\t\t\t\t\t\n", len(groups)-top)
			break
		}
		line := fmt.Sprintf("%s\t%d\t%d\t%d\t%.6f\t%.9f",
			truncate(g.Key, 42), g.Requests, g.Successful, g.FulfilmentFailed, g.FeesEarnedXfund, g.GasCostEth)
		if xfundPriceEth > 0 {
			line += fmt.Sprintf("\t%.9f", g.FeesEarnedXfund*xfundPriceEth-g.GasCostEth)
		}
		fmt.Fprintln(w, line)
	}
	w.Flush()
}

func printFailures(failures []report.Failure, top int) {
	fmt.Printf("\nFailures by reason (%d)\n", len(failures))
	if len(failures) == 0 {
		fmt.Println("  (none)")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "count\treason")
	for i, f := range failures {
		if top > 0 && i >= top {
			fmt.Fprintf(w, "\t... and %d more\n", len(failures)-top)
			break
		}
		fmt.Fprintf(w, "%d\t%s\n", f.Count, f.Reason)
	}
	w.Flush()
}

// pct renders a success rate, guarding the "no terminal outcomes yet" case so it reads clearly
// rather than as a misleading 0.0%.
func pct(rate float64, terminal int) string {
	if terminal == 0 {
		return "n/a (no completed requests yet)"
	}
	return fmt.Sprintf("%.1f%%", rate)
}

// truncate keeps wide keys (addresses) from blowing out the table width.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

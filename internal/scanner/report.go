package scanner

import (
	"fmt"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// PrintReport writes a human-readable report to stdout.
func PrintReport(r *ScanResult) {
	fmt.Printf("\n%s%s milvscan :: %s%s\n", colorBold, colorCyan, r.Target, colorReset)
	fmt.Printf("%s%s  [%s]%s\n\n", colorDim, r.ScanTime.UTC().Format("2006-01-02T15:04:05Z"), r.APIVersion, colorReset)

	// Meta
	authStr := fmt.Sprintf("%sOPEN (no auth)%s", colorRed, colorReset)
	switch r.AuthStatus {
	case "AUTH_REQUIRED":
		authStr = fmt.Sprintf("%sAUTH REQUIRED%s", colorGreen, colorReset)
	case "CATCH_ALL":
		authStr = fmt.Sprintf("%sCATCH-ALL DECOY (fingerprint poisoning)%s", colorYellow, colorReset)
	case "UNKNOWN":
		authStr = fmt.Sprintf("%sUNKNOWN (not Milvus or port unreachable)%s", colorDim, colorReset)
	}
	fmt.Printf("## Meta\n")
	if r.Version != "" {
		fmt.Printf("  version    : %s\n", r.Version)
	}
	fmt.Printf("  auth       : %s\n\n", authStr)

	// Collections
	if len(r.Collections) > 0 {
		fmt.Printf("## Collections (%d)\n", len(r.Collections))
		fmt.Printf("  %-28s  %10s  %6s  %5s  %s\n", "COLLECTION", "ROWS", "DIM", "SCORE", "SEVERITY")
		fmt.Printf("  %s\n", strings.Repeat("-", 70))
		for _, cr := range r.Collections {
			scoreColor := colorReset
			switch cr.Severity {
			case "CRITICAL", "HIGH":
				scoreColor = colorRed
			case "MEDIUM":
				scoreColor = colorYellow
			}
			tags := ""
			if len(cr.PIISignals) > 0 {
				tags += fmt.Sprintf(" [PII: %s]", strings.Join(cr.PIISignals, ","))
			}
			fmt.Printf("  %-28s  %10d  %6d  %s%5.1f%s  %s%s\n",
				truncate(cr.Name, 28),
				cr.RowCount,
				cr.VectorDim,
				scoreColor, cr.Score, colorReset,
				cr.Severity,
				tags,
			)
		}
		fmt.Println()
	}

	// Schema / field keys
	for _, cr := range r.Collections {
		if len(cr.Fields) > 0 {
			fmt.Printf("## Schema: %s\n", cr.Name)
			for _, f := range cr.Fields {
				pk := ""
				if f.Primary {
					pk = " [PK]"
				}
				dim := ""
				if f.Dimension > 0 {
					dim = fmt.Sprintf(" dim=%d", f.Dimension)
				}
				fmt.Printf("  %-24s  %s%s%s\n", f.Name, f.Type, pk, dim)
			}
			fmt.Println()
		}
		if len(cr.PayloadKeys) > 0 {
			fmt.Printf("## Record keys: %s\n", cr.Name)
			fmt.Printf("  %s\n\n", strings.Join(cr.PayloadKeys, ", "))
		}
	}

	// Canary
	if r.Canary != nil && r.Canary.Attempted {
		fmt.Printf("## Write Canary\n")
		if r.Canary.Success {
			fmt.Printf("  %s[!] WRITE ACCESS CONFIRMED  collection=%s%s\n",
				colorRed, r.Canary.Collection, colorReset)
		} else {
			fmt.Printf("  write test failed: %s\n", r.Canary.Error)
		}
		fmt.Println()
	}

	// Summary
	sevColor := colorReset
	switch r.Severity {
	case "CRITICAL", "HIGH":
		sevColor = colorRed
	case "MEDIUM":
		sevColor = colorYellow
	}

	fmt.Printf("## Summary\n")
	fmt.Printf("  severity   : %s%s%s%s\n", colorBold, sevColor, r.Severity, colorReset)
	fmt.Printf("  score      : %.1f\n", r.Score)
	fmt.Printf("  auth       : %s\n", r.AuthStatus)
	fmt.Printf("  collections: %d total\n", r.Summary.TotalCollections)
	fmt.Printf("  rows       : ~%d estimated\n", r.Summary.TotalRowsEstimated)
	fmt.Printf("  pii hits   : %d collections\n", r.Summary.PIICollections)
	fmt.Printf("  findings   : %d extracted\n", r.Summary.FindingsExtracted)
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

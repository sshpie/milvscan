package scanner

import "fmt"

// Scanner orchestrates all scan phases.
type Scanner struct {
	cfg *Config
	c   *httpClient
}

// New constructs a Scanner from cfg.
func New(cfg *Config) *Scanner {
	return &Scanner{
		cfg: cfg,
		c:   newClient(cfg.Target, cfg.Token, cfg.TimeoutSec, cfg.Verbose),
	}
}

// Run executes the scan pipeline and populates result in place.
func (s *Scanner) Run(result *ScanResult) error {
	cfg := s.cfg
	c := s.c

	fmt.Printf("[0] probe %s\n", cfg.Target)

	pr, err := probe(c)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	result.APIVersion = apiVersionString(pr.apiVer)
	result.Version = pr.version
	result.AuthStatus = pr.authStatus

	if pr.authStatus == "AUTH_REQUIRED" {
		fmt.Println("    auth required -- stopping")
		buildSummary(result)
		return nil
	}
	if pr.authStatus == "CATCH_ALL" {
		fmt.Println("    catch-all decoy (bogus path 200) -- not a real Milvus, stopping")
		buildSummary(result)
		return nil
	}
	if pr.authStatus == "UNKNOWN" {
		fmt.Println("    not Milvus or port unreachable -- stopping")
		buildSummary(result)
		return nil
	}

	if cfg.ProbeOnly {
		buildSummary(result)
		return nil
	}

	authOpen := pr.authStatus == "OPEN"

	// Phase 1: list collections
	fmt.Println("[1] listing collections")
	names, err := listCollections(c, pr.apiVer)
	if err != nil {
		fmt.Printf("    collection list error: %v\n", err)
		buildSummary(result)
		return nil
	}
	fmt.Printf("    %d collections found\n", len(names))

	// Phase 2: describe + sample each collection
	fmt.Printf("[2] enumerating %d collections\n", len(names))
	for _, name := range names {
		cr := enumerateCollection(c, name, pr.apiVer, authOpen, cfg.SampleLimit, cfg.Verbose)
		result.Collections = append(result.Collections, cr)
	}

	// Write canary (optional, requires --authorize-write gate in main.go)
	if cfg.DoWriteCanary && len(result.Collections) > 0 {
		fmt.Println("[W] canary write")
		target := result.Collections[0]
		canary := writeCanary(c, target.Name, target.Fields, pr.apiVer)
		result.Canary = canary
		if canary.Success {
			result.WriteTest = "SUCCESS"
			rescoreWithWrite(result.Collections)
		} else {
			result.WriteTest = "FAILED"
		}
	}

	buildSummary(result)
	return nil
}

// enumerateCollection fetches schema, counts, samples, and scores one collection.
func enumerateCollection(c *httpClient, name string, ver apiVersion, authOpen bool, sampleLimit int, verbose bool) CollectionResult {
	cr := describeCollection(c, name, ver)

	records, err := sampleRecords(c, name, sampleLimit, ver)
	if err != nil {
		if verbose {
			fmt.Printf("    sample %s: %v\n", name, err)
		}
	} else {
		cr.SampleRecords = records
		cr.PayloadKeys = recordKeys(records)
		cr.PIISignals = scanRecords(records)
	}

	cr.Score = scoreCollection(authOpen, cr.PIISignals, cr.RowCount, false)
	cr.Severity = severityLabel(cr.Score)
	return cr
}

// rescoreWithWrite re-scores all collections once write access is confirmed.
func rescoreWithWrite(cols []CollectionResult) {
	for i := range cols {
		cols[i].Score = scoreCollection(true, cols[i].PIISignals, cols[i].RowCount, true)
		cols[i].Severity = severityLabel(cols[i].Score)
	}
}

// buildSummary rolls up collection-level data into the top-level result.
func buildSummary(r *ScanResult) {
	r.Score = overallScore(r.Collections)
	r.Severity = severityLabel(r.Score)

	sum := &r.Summary
	sum.AuthStatus = r.AuthStatus
	sum.TotalCollections = len(r.Collections)
	for _, cr := range r.Collections {
		sum.TotalRowsEstimated += cr.RowCount
		if len(cr.PIISignals) > 0 {
			sum.PIICollections++
		}
		sum.FindingsExtracted += len(cr.SampleRecords)
	}
	sum.Severity = r.Severity
}

// apiVersionString converts the internal apiVersion enum to a display string.
func apiVersionString(v apiVersion) string {
	switch v {
	case apiV1:
		return "v1 (/v1/vector/*)"
	case apiV2:
		return "v2 (/v2/vectordb/*)"
	default:
		return "unknown"
	}
}

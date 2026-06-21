package scanner

// scoreCollection computes a severity score for a single Milvus collection.
//
// Scoring table:
//
//	AUTH_OFF + collection reachable   +4.0
//	PII detected in records           +3.0
//	Write confirmed                   +1.5 (per-collection rescore after canary)
//	Large corpus (>10k rows)          +0.5
//
// Note: Milvus has no REST snapshot API, so the +2.0 snapshot dimension from
// qdrantscan/chromascan is absent here.
func scoreCollection(authOpen bool, piiSignals []string, rowCount int64, writeConfirmed bool) float64 {
	var s float64
	if authOpen {
		s += 4.0
	}
	if len(piiSignals) > 0 {
		s += 3.0
	}
	if writeConfirmed {
		s += 1.5
	}
	if rowCount > 10000 {
		s += 0.5
	}
	return s
}

// severityLabel maps a numeric score to a human-readable severity label.
func severityLabel(score float64) string {
	switch {
	case score >= 8.0:
		return "CRITICAL"
	case score >= 6.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// overallScore returns the highest per-collection score as the scan score.
func overallScore(cols []CollectionResult) float64 {
	var max float64
	for _, c := range cols {
		if c.Score > max {
			max = c.Score
		}
	}
	return max
}

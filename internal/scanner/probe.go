package scanner

import (
	"fmt"
	"strings"
)

// decoyPath is a path that no real Milvus instance should return 200 on.
// If we get 200 back, this is a catch-all decoy poisoning the scanner population.
const decoyPath = "/milvscan-decoy-7x9k"

// probeResult carries everything learned in the probe phase.
type probeResult struct {
	apiVer     apiVersion
	authStatus string // OPEN | AUTH_REQUIRED | CATCH_ALL | UNKNOWN
	version    string
}

// probe determines the API version, auth status, and Milvus version string.
// It tries v2 first (Milvus 2.4+, /v2/vectordb/) then falls back to v1
// (Milvus 2.2-2.3, /v1/vector/).
func probe(c *httpClient) (*probeResult, error) {
	pr := &probeResult{}

	// 1. Catch-all decoy check. A real Milvus 404s on unknown paths.
	status, _, _ := c.getRaw(decoyPath)
	if status == 200 {
		pr.authStatus = "CATCH_ALL"
		return pr, nil
	}

	// 2. Try v2 REST API (Milvus 2.4+)
	var v2r v2ListResp
	status, _ = c.post("/v2/vectordb/collections/list", map[string]interface{}{}, &v2r)
	if status == 401 || status == 403 {
		pr.apiVer = apiV2
		pr.authStatus = "AUTH_REQUIRED"
		pr.version = detectVersion(c)
		return pr, nil
	}
	if status == 200 {
		if v2r.Code == 0 {
			// Validate against selective decoys: real Milvus always has milvus_* metric families.
			if !metricsReal(c) {
				pr.authStatus = "CATCH_ALL"
				return pr, nil
			}
			pr.apiVer = apiV2
			pr.authStatus = "OPEN"
			pr.version = detectVersion(c)
			fmt.Printf("    api v2, collections=%d\n", len(v2r.Data))
			return pr, nil
		}
		// Non-zero code = auth or error
		if isAuthCode(v2r.Code) {
			pr.apiVer = apiV2
			pr.authStatus = "AUTH_REQUIRED"
			pr.version = detectVersion(c)
			return pr, nil
		}
	}

	// 3. Try v1 REST API (Milvus 2.2-2.3)
	var v1r v1ListResp
	status, _ = c.get("/v1/vector/collections", &v1r)
	if status == 401 || status == 403 {
		pr.apiVer = apiV1
		pr.authStatus = "AUTH_REQUIRED"
		pr.version = detectVersion(c)
		return pr, nil
	}
	if status == 200 {
		if v1r.Code == 200 {
			if !metricsReal(c) {
				pr.authStatus = "CATCH_ALL"
				return pr, nil
			}
			pr.apiVer = apiV1
			pr.authStatus = "OPEN"
			pr.version = detectVersion(c)
			fmt.Printf("    api v1, collections=%d\n", len(v1r.Data))
			return pr, nil
		}
		if isAuthCode(v1r.Code) {
			pr.apiVer = apiV1
			pr.authStatus = "AUTH_REQUIRED"
			pr.version = detectVersion(c)
			return pr, nil
		}
	}

	// 4. Try healthz as last resort to confirm it's even Milvus.
	var hz healthzResp
	hzStatus, _ := c.get("/healthz", &hz)
	if hzStatus == 200 && strings.Contains(strings.ToLower(hz.Status), "healthy") {
		pr.authStatus = "UNKNOWN"
		return pr, nil
	}

	pr.authStatus = "UNKNOWN"
	return pr, nil
}

// isAuthCode returns true for Milvus error codes that indicate auth failure.
// Milvus v1: 1800 = "auth check failure". v2: 65535 = unauthenticated.
func isAuthCode(code int) bool {
	return code == 1800 || code == 65535 || code == 80
}

// metricsReal checks /metrics for milvus_* metric families -- the Cat-13
// discriminator for real vs selective decoy. Selective decoys respond correctly
// on known API paths but either have no /metrics at all or return 0 milvus_*
// metric families. A real Milvus instance always has milvus_* counters in /metrics.
// Returns true when real milvus metrics are found, false for decoy or unreachable.
func metricsReal(c *httpClient) bool {
	_, body, err := c.getRaw("/metrics")
	if err != nil || len(body) == 0 {
		return false
	}
	return strings.Contains(string(body), "milvus_")
}

// detectVersion reads the Milvus version from /v2/vectordb/milvus/version (v2)
// or /v1/vector/version (v1). Returns empty string when unavailable.
func detectVersion(c *httpClient) string {
	type versionResp struct {
		Code int    `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	var vr versionResp
	// Try v2 version endpoint.
	status, _ := c.post("/v2/vectordb/milvus/version", map[string]interface{}{}, &vr)
	if status == 200 && (vr.Code == 0 || vr.Code == 200) && vr.Data.Version != "" {
		return vr.Data.Version
	}
	// Try v1.
	type v1VersionResp struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}
	var vr1 v1VersionResp
	status, _ = c.get("/v1/vector/version", &vr1)
	if status == 200 && (vr1.Code == 0 || vr1.Code == 200) && vr1.Data != "" {
		return vr1.Data
	}
	return ""
}

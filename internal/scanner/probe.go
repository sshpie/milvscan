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

// metricsReal is the Cat-13 discriminator for real Milvus vs selective decoy.
//
// Port 9091 (management): /metrics returns 200 with milvus_* Prometheus families.
// Port 19530 (REST+gRPC): /metrics returns 404 -- this port does not serve Prometheus.
//
// Strategy:
//   - 200 with milvus_* in body -> real (9091 case)
//   - 200 without milvus_* -> decoy (9091 selective decoy case)
//   - 404 -> fall back to version-endpoint discriminator (19530 case)
//   - error/empty body -> false (conservative)
func metricsReal(c *httpClient) bool {
	status, body, err := c.getRaw("/metrics")
	if err != nil {
		return false
	}
	if status == 200 {
		return len(body) > 0 && strings.Contains(string(body), "milvus_")
	}
	if status == 404 {
		// Port doesn't serve /metrics (typical for port 19530). Fall back to
		// the version endpoint as the discriminator: a selective decoy is unlikely
		// to implement every Milvus-specific API path correctly.
		return versionEndpointReal(c)
	}
	return false
}

// versionEndpointReal is the fallback discriminator for port 19530 (where /metrics
// is not exposed). Cascade:
//   1. POST /v2/vectordb/milvus/version -> version string present = real
//   2. POST /v2/vectordb/collections/describe with a nonexistent collection name:
//      code != 0 = real (Milvus error); code == 0 = catch-all decoy
//   3. Cannot discriminate -> assume real (conservative)
func versionEndpointReal(c *httpClient) bool {
	type vResp struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	var vr vResp
	vStatus, _ := c.post("/v2/vectordb/milvus/version", map[string]interface{}{}, &vr)
	if vStatus == 200 && vr.Code == 0 && strings.HasPrefix(vr.Data.Version, "v") {
		return true
	}

	// Ghost-describe: POST describe with a collection name guaranteed not to exist.
	// Milvus returns code != 0 ("can't find collection"). A catch-all decoy returns code == 0.
	// Collection names in Milvus are alphanumeric+underscore only.
	type descResp struct {
		Code int `json:"code"`
	}
	var dr descResp
	dStatus, _ := c.post("/v2/vectordb/collections/describe",
		map[string]interface{}{"collectionName": "milvscan_ghost_xyz9"}, &dr)
	if dStatus == 200 {
		return dr.Code != 0 // real Milvus returns non-zero for nonexistent collection
	}

	// Cannot discriminate; assume real (conservative -- prefer false positive over miss).
	return true
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

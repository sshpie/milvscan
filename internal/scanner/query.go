package scanner

import "fmt"

// sampleRecords fetches up to limit entity records from a Milvus collection.
// Vectors are excluded -- we only want the scalar payload fields.
func sampleRecords(c *httpClient, name string, limit int, ver apiVersion) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 5
	}
	switch ver {
	case apiV2:
		return sampleV2(c, name, limit)
	default:
		return sampleV1(c, name, limit)
	}
}

// sampleV1 queries via POST /v1/vector/query.
// Expression "id >= 0" returns all entities (Milvus requires a filter expression).
func sampleV1(c *httpClient, name string, limit int) ([]map[string]interface{}, error) {
	body := map[string]interface{}{
		"collectionName": name,
		"filter":         "id >= 0",
		"limit":          limit,
		"outputFields":   []string{"*"},
	}
	var r v1QueryResp
	status, err := c.post("/v1/vector/query", body, &r)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	if r.Code != 200 {
		// Try without the filter (older releases accept empty filter).
		body2 := map[string]interface{}{
			"collectionName": name,
			"limit":          limit,
			"outputFields":   []string{"*"},
		}
		var r2 v1QueryResp
		_, err2 := c.post("/v1/vector/query", body2, &r2)
		if err2 != nil || r2.Code != 200 {
			return nil, fmt.Errorf("v1 query code=%d msg=%s", r.Code, r.Message)
		}
		return r2.Data, nil
	}
	return r.Data, nil
}

// sampleV2 queries via POST /v2/vectordb/entities/query.
func sampleV2(c *httpClient, name string, limit int) ([]map[string]interface{}, error) {
	body := map[string]interface{}{
		"collectionName": name,
		"filter":         "id >= 0",
		"limit":          limit,
		"outputFields":   []string{"*"},
	}
	var r v2QueryResp
	status, err := c.post("/v2/vectordb/entities/query", body, &r)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	if r.Code != 0 {
		// Retry without filter.
		body2 := map[string]interface{}{
			"collectionName": name,
			"limit":          limit,
			"outputFields":   []string{"*"},
		}
		var r2 v2QueryResp
		_, err2 := c.post("/v2/vectordb/entities/query", body2, &r2)
		if err2 != nil || r2.Code != 0 {
			return nil, fmt.Errorf("v2 query code=%d msg=%s", r.Code, r.Message)
		}
		return r2.Data, nil
	}
	return r.Data, nil
}

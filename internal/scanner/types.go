package scanner

import "time"

// Config holds all runtime configuration.
type Config struct {
	Target        string
	Token         string // Bearer token for authed instances
	TimeoutSec    int
	DoWriteCanary bool
	ProbeOnly     bool
	SampleLimit   int
	OutputFile    string
	Verbose       bool
}

// apiVersion is which Milvus REST API was detected.
type apiVersion int

const (
	apiUnknown apiVersion = iota
	apiV1                 // /v1/vector/* (Milvus 2.2-2.3, port 19121)
	apiV2                 // /v2/vectordb/* (Milvus 2.4+, port 9530)
)

// Raw Milvus API response envelopes.
//
// v1 success: {"code":200,"data":<T>,"message":""}
// v2 success: {"code":0,"data":<T>}
// error:      {"code":<N>,"message":"..."}

type v1ListResp struct {
	Code    int      `json:"code"`
	Data    []string `json:"data"`
	Message string   `json:"message"`
}

type v2ListResp struct {
	Code    int      `json:"code"`
	Data    []string `json:"data"`
	Message string   `json:"message"`
}

// v1DescribeResp wraps GET /v1/vector/collections/describe?collectionName=X
type v1DescribeResp struct {
	Code    int            `json:"code"`
	Data    v1CollInfo     `json:"data"`
	Message string         `json:"message"`
}

type v1CollInfo struct {
	CollectionName string       `json:"collectionName"`
	Fields         []v1Field    `json:"fields"`
	RowCount       int64        `json:"rowCount"`
	Status         string       `json:"status"`
}

type v1Field struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	PrimaryKey bool                   `json:"primaryKey"`
	AutoID     bool                   `json:"autoId"`
	Params     map[string]interface{} `json:"params"`
}

// v2DescribeResp wraps POST /v2/vectordb/collections/describe
type v2DescribeResp struct {
	Code    int        `json:"code"`
	Data    v2CollInfo `json:"data"`
	Message string     `json:"message"`
}

type v2CollInfo struct {
	CollectionName string    `json:"collectionName"`
	Fields         []v2Field `json:"fields"`
	RowCount       int64     `json:"rowCount"`
}

// v2Param is one entry in the "params" array in a v2 field describe response.
// Milvus returns params as [{"key":"dim","value":"1536"}], not as a map.
type v2Param struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type v2Field struct {
	Name      string    `json:"name"`
	DataType  string    `json:"type"`
	IsPrimary bool      `json:"primaryKey"`
	AutoID    bool      `json:"autoId"`
	Params    []v2Param `json:"params"`
}

// v1QueryResp wraps POST /v1/vector/query
type v1QueryResp struct {
	Code    int                      `json:"code"`
	Data    []map[string]interface{} `json:"data"`
	Message string                   `json:"message"`
}

// v2QueryResp wraps POST /v2/vectordb/entities/query
type v2QueryResp struct {
	Code    int                      `json:"code"`
	Data    []map[string]interface{} `json:"data"`
	Message string                   `json:"message"`
}

// v1InsertResp wraps POST /v1/vector/insert (write canary)
type v1InsertResp struct {
	Code    int    `json:"code"`
	Data    struct {
		InsertCount int `json:"insertCount"`
	} `json:"data"`
	Message string `json:"message"`
}

// v1DeleteResp wraps POST /v1/vector/delete (canary cleanup)
type v1DeleteResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// healthzResp is the /healthz response.
type healthzResp struct {
	Status string `json:"status"`
}

// Output types

// FieldInfo records one Milvus field (schema).
type FieldInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Primary   bool   `json:"primary,omitempty"`
	Dimension int    `json:"dimension,omitempty"`
}

// CollectionResult holds enumeration output for one Milvus collection.
type CollectionResult struct {
	Name           string                   `json:"name"`
	RowCount       int64                    `json:"row_count"`
	Status         string                   `json:"status,omitempty"`
	Fields         []FieldInfo              `json:"fields,omitempty"`
	VectorDim      int                      `json:"vector_dim,omitempty"`
	PayloadKeys    []string                 `json:"payload_keys,omitempty"`
	PIISignals     []string                 `json:"pii_signals,omitempty"`
	SampleRecords  []map[string]interface{} `json:"sample_records,omitempty"`
	Score          float64                  `json:"score"`
	Severity       string                   `json:"severity"`
}

// CanaryResult records the write canary outcome.
type CanaryResult struct {
	Attempted  bool   `json:"attempted"`
	Success    bool   `json:"success"`
	Collection string `json:"collection,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ScanResult is the top-level output object.
type ScanResult struct {
	Target      string             `json:"target"`
	ScanTime    time.Time          `json:"scan_time"`
	APIVersion  string             `json:"api_version"`
	Version     string             `json:"version,omitempty"`
	AuthStatus  string             `json:"auth_status"`
	Collections []CollectionResult `json:"collections,omitempty"`
	WriteTest   string             `json:"write_test,omitempty"`
	Canary      *CanaryResult      `json:"canary,omitempty"`
	Score       float64            `json:"score"`
	Severity    string             `json:"severity"`
	Summary     ScanSummary        `json:"summary"`
}

// ScanSummary is the rolled-up finding counts.
type ScanSummary struct {
	TotalCollections     int    `json:"total_collections"`
	TotalRowsEstimated   int64  `json:"total_rows_estimated"`
	PIICollections       int    `json:"pii_collections"`
	FindingsExtracted    int    `json:"findings_extracted"`
	Severity             string `json:"severity"`
	AuthStatus           string `json:"auth_status"`
}

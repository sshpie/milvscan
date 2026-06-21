package scanner

import (
	"fmt"
	"strconv"
)

// listCollections returns the collection names for the detected API version.
func listCollections(c *httpClient, ver apiVersion) ([]string, error) {
	switch ver {
	case apiV2:
		var r v2ListResp
		_, err := c.post("/v2/vectordb/collections/list", map[string]interface{}{}, &r)
		if err != nil {
			return nil, err
		}
		if r.Code != 0 {
			return nil, fmt.Errorf("v2 list code=%d msg=%s", r.Code, r.Message)
		}
		return r.Data, nil
	default: // apiV1
		var r v1ListResp
		_, err := c.get("/v1/vector/collections", &r)
		if err != nil {
			return nil, err
		}
		if r.Code != 200 {
			return nil, fmt.Errorf("v1 list code=%d msg=%s", r.Code, r.Message)
		}
		return r.Data, nil
	}
}

// describeCollection fetches schema and row count for a single collection.
// Returns a CollectionResult pre-populated with schema info.
func describeCollection(c *httpClient, name string, ver apiVersion) CollectionResult {
	cr := CollectionResult{Name: name}

	switch ver {
	case apiV2:
		var r v2DescribeResp
		_, err := c.post("/v2/vectordb/collections/describe",
			map[string]interface{}{"collectionName": name}, &r)
		if err != nil {
			return cr
		}
		if r.Code != 0 {
			return cr
		}
		cr.RowCount = r.Data.RowCount
		for _, f := range r.Data.Fields {
			fi := FieldInfo{Name: f.Name, Type: f.DataType, Primary: f.IsPrimary}
			if dim, ok := f.Params["dim"]; ok {
				fi.Dimension = parseDim(dim)
				if fi.Dimension > 0 && cr.VectorDim == 0 {
					cr.VectorDim = fi.Dimension
				}
			}
			cr.Fields = append(cr.Fields, fi)
		}

	default: // apiV1
		var r v1DescribeResp
		_, err := c.get("/v1/vector/collections/describe?collectionName="+name, &r)
		if err != nil {
			return cr
		}
		if r.Code != 200 {
			return cr
		}
		cr.RowCount = r.Data.RowCount
		cr.Status = r.Data.Status
		for _, f := range r.Data.Fields {
			fi := FieldInfo{Name: f.Name, Type: f.Type, Primary: f.PrimaryKey}
			if dim, ok := f.Params["dim"]; ok {
				fi.Dimension = parseDim(dim)
				if fi.Dimension > 0 && cr.VectorDim == 0 {
					cr.VectorDim = fi.Dimension
				}
			}
			cr.Fields = append(cr.Fields, fi)
		}
	}

	return cr
}

// parseDim converts the "dim" parameter to an int. Milvus can return it as
// either a number or a string in the JSON depending on SDK version.
func parseDim(v interface{}) int {
	switch d := v.(type) {
	case float64:
		return int(d)
	case string:
		n, _ := strconv.Atoi(d)
		return n
	}
	return 0
}

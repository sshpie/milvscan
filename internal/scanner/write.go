package scanner

import "fmt"

const canaryPKValue int64 = 9876543219999999

// writeCanary inserts a clearly-labeled entity and immediately deletes it.
// Requires the target collection's field schema to build a valid entity.
// Cleans up unconditionally -- the insert proves write access; the delete
// removes the artifact. Returns the canary result regardless of cleanup success.
func writeCanary(c *httpClient, collName string, fields []FieldInfo, ver apiVersion) *CanaryResult {
	cr := &CanaryResult{Attempted: true, Collection: collName}

	entity, pkField, err := buildCanaryEntity(fields)
	if err != nil {
		cr.Error = "schema too complex: " + err.Error()
		return cr
	}

	switch ver {
	case apiV2:
		err = insertV2(c, collName, entity)
	default:
		err = insertV1(c, collName, entity)
	}
	if err != nil {
		cr.Error = fmt.Sprintf("insert failed: %v", err)
		return cr
	}

	cr.Success = true

	// Delete by primary key, best-effort.
	switch ver {
	case apiV2:
		deleteV2(c, collName, pkField)
	default:
		deleteV1(c, collName, pkField)
	}

	return cr
}

func insertV1(c *httpClient, collName string, entity map[string]interface{}) error {
	body := map[string]interface{}{
		"collectionName": collName,
		"data":           []interface{}{entity},
	}
	var r v1InsertResp
	_, err := c.post("/v1/vector/insert", body, &r)
	if err != nil {
		return err
	}
	if r.Code != 200 {
		return fmt.Errorf("code=%d msg=%s", r.Code, r.Message)
	}
	return nil
}

func insertV2(c *httpClient, collName string, entity map[string]interface{}) error {
	body := map[string]interface{}{
		"collectionName": collName,
		"data":           []interface{}{entity},
	}
	type insertResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	var r insertResp
	_, err := c.post("/v2/vectordb/entities/insert", body, &r)
	if err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("code=%d msg=%s", r.Code, r.Message)
	}
	return nil
}

func deleteV1(c *httpClient, collName, pkField string) {
	body := map[string]interface{}{
		"collectionName": collName,
		"filter":         fmt.Sprintf("%s == %d", pkField, canaryPKValue),
	}
	c.post("/v1/vector/delete", body, nil) //nolint:errcheck
}

func deleteV2(c *httpClient, collName, pkField string) {
	body := map[string]interface{}{
		"collectionName": collName,
		"filter":         fmt.Sprintf("%s == %d", pkField, canaryPKValue),
	}
	c.post("/v2/vectordb/entities/delete", body, nil) //nolint:errcheck
}

// buildCanaryEntity constructs a minimal valid entity from the collection schema.
// Returns the entity map, the primary-key field name, and an error if the schema
// is unhandleable (e.g., no vector field found).
func buildCanaryEntity(fields []FieldInfo) (map[string]interface{}, string, error) {
	entity := map[string]interface{}{}
	pkField := "id"
	hasVector := false

	for _, f := range fields {
		switch f.Type {
		case "Int64", "INT64":
			if f.Primary {
				pkField = f.Name
				entity[f.Name] = canaryPKValue
			} else {
				entity[f.Name] = int64(0)
			}
		case "Int32", "INT32", "Int16", "INT16", "Int8", "INT8":
			entity[f.Name] = int32(0)
		case "Float", "FLOAT", "Double", "DOUBLE":
			entity[f.Name] = float64(0)
		case "Bool", "BOOL":
			entity[f.Name] = false
		case "VarChar", "VARCHAR", "String", "STRING":
			if f.Primary {
				pkField = f.Name
				entity[f.Name] = fmt.Sprintf("nuclide-canary-%d", canaryPKValue)
			} else {
				entity[f.Name] = "nuclide-canary"
			}
		case "FloatVector", "FLOAT_VECTOR":
			dim := f.Dimension
			if dim == 0 {
				dim = 8
			}
			vec := make([]float32, dim)
			entity[f.Name] = vec
			hasVector = true
		case "BFloat16Vector", "BFLOAT16_VECTOR",
			"Float16Vector", "FLOAT16_VECTOR",
			"BinaryVector", "BINARY_VECTOR":
			// Skip exotic vector types.
		case "SparseFloatVector", "SPARSE_FLOAT_VECTOR":
			entity[f.Name] = map[string]interface{}{}
			hasVector = true
		case "JSON", "json":
			entity[f.Name] = map[string]interface{}{"nuclide": true}
		case "Array", "ARRAY":
			entity[f.Name] = []interface{}{}
		}
	}

	if !hasVector && len(fields) > 0 {
		return nil, "", fmt.Errorf("no FloatVector field found in schema")
	}
	return entity, pkField, nil
}

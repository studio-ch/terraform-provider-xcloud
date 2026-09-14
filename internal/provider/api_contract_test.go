package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Snapshot exported from the public API; retained here so isolated provider tests validate the same contract.
func loadPublicContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/public-api.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]any `json:"paths"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Paths
}
func checkPublicRequest(paths map[string]any, r *http.Request, body map[string]any) error {
	if paths == nil {
		return nil
	}
	operation := publicOperation(paths, r)
	if operation == nil {
		return fmt.Errorf("%s %s is not declared in the public OpenAPI contract", r.Method, r.URL.Path)
	}
	queries := map[string]bool{}
	if params, ok := operation["parameters"].([]any); ok {
		for _, p := range params {
			param := p.(map[string]any)
			if param["in"] == "query" {
				queries[param["name"].(string)] = true
			}
			if param["in"] == "query" && param["required"] == true && r.URL.Query().Get(param["name"].(string)) == "" {
				return fmt.Errorf("missing query parameter %s", param["name"])
			}
		}
	}
	for name := range r.URL.Query() {
		if !queries[name] {
			return fmt.Errorf("undeclared query parameter %s", name)
		}
	}
	rb, ok := operation["requestBody"].(map[string]any)
	if !ok {
		return nil
	}
	content, _ := rb["content"].(map[string]any)
	application, _ := content["application/json"].(map[string]any)
	s, _ := application["schema"].(map[string]any)
	return validatePublicValue(s, body, "body")
}
func validatePublicValue(s map[string]any, v any, location string) error {
	if len(s) == 0 {
		return nil
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		if variants, ok := s[keyword].([]any); ok {
			for _, variant := range variants {
				if validatePublicValue(variant.(map[string]any), v, location) == nil {
					return nil
				}
			}
			return fmt.Errorf("%s does not match any %s variant", location, keyword)
		}
	}
	if variants, ok := s["type"].([]any); ok {
		for _, typ := range variants {
			copy := map[string]any{}
			for k, v := range s {
				copy[k] = v
			}
			copy["type"] = typ
			if validatePublicValue(copy, v, location) == nil {
				return nil
			}
		}
		return fmt.Errorf("%s does not match allowed types", location)
	}
	if v == nil {
		if s["nullable"] == true || s["type"] == "null" {
			return nil
		}
		return fmt.Errorf("%s unexpectedly null", location)
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", location)
		}
		if req, ok := s["required"].([]any); ok {
			for _, field := range req {
				if _, ok := obj[field.(string)]; !ok {
					return fmt.Errorf("missing required %s.%s", location, field)
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		for key, value := range obj {
			field, ok := props[key].(map[string]any)
			if !ok {
				if extra, ok := s["additionalProperties"].(map[string]any); ok {
					field = extra
				} else if s["additionalProperties"] == true {
					continue
				} else {
					return fmt.Errorf("undeclared public API field %s.%s", location, key)
				}
			}
			if err := validatePublicValue(field, value, location+"."+key); err != nil {
				return err
			}
		}
	case "array":
		values, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", location)
		}
		items, _ := s["items"].(map[string]any)
		for i, value := range values {
			if err := validatePublicValue(items, value, fmt.Sprintf("%s[%d]", location, i)); err != nil {
				return err
			}
		}
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%s must be a string", location)
		}
	case "integer", "number":
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("%s must be a number", location)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", location)
		}
	}
	if values, ok := s["enum"].([]any); ok {
		found := false
		for _, allowed := range values {
			if reflect.DeepEqual(v, allowed) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s has invalid enum value %v", location, v)
		}
	}
	return nil
}

func publicOperation(paths map[string]any, r *http.Request) map[string]any {
	var operation map[string]any
	requestParts := strings.Split(r.URL.Path, "/")
	for template, methods := range paths {
		templateParts := strings.Split(template, "/")
		if len(templateParts) != len(requestParts) {
			continue
		}
		match := true
		for i, p := range templateParts {
			if !(strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}")) && p != requestParts[i] {
				match = false
				break
			}
		}
		if match {
			operation, _ = methods.(map[string]any)[strings.ToLower(r.Method)].(map[string]any)
			if operation != nil {
				break
			}
		}
	}

	return operation
}

// Enforce declared HTTP status and JSON-vs-empty response semantics for every
// successful mock response. DTO values are exercised by Terraform state checks;
// this is deliberately not a claim of full response-schema validation.
func checkPublicResponse(paths map[string]any, r *http.Request, response *httptest.ResponseRecorder) error {
	if response.Code >= 400 {
		return nil
	}
	op := publicOperation(paths, r)
	if op == nil {
		return fmt.Errorf("undeclared response route %s %s", r.Method, r.URL.Path)
	}
	responses, _ := op["responses"].(map[string]any)
	declared, ok := responses[strconv.Itoa(response.Code)].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s: undeclared success status %d", r.Method, r.URL.Path, response.Code)
	}
	content, _ := declared["content"].(map[string]any)
	_, wantsJSON := content["application/json"]
	body := bytes.TrimSpace(response.Body.Bytes())
	if wantsJSON && !json.Valid(body) {
		return fmt.Errorf("%s %s: declared JSON response is invalid or empty", r.Method, r.URL.Path)
	}
	if !wantsJSON && len(body) != 0 {
		return fmt.Errorf("%s %s: expected an empty response", r.Method, r.URL.Path)
	}
	return nil
}

func TestPublicQueryParameterNames(t *testing.T) {
	contract := loadPublicContract(t)
	for _, tc := range []struct {
		method, path string
		valid        bool
	}{
		{"DELETE", "/v1/xcloud/instances/id?releaseElasticIps=false", true},
		{"DELETE", "/v1/xcloud/instances/id?releaseElasticIPs=false", false},
		{"GET", "/v1/xcloud/networks?regionId=region", true},
		{"GET", "/v1/xcloud/networks?region_id=region", false},
		{"GET", "/v1/ssh-keys?regionId=region", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			err := checkPublicRequest(contract, httptest.NewRequest(tc.method, tc.path, nil), nil)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, got %v", tc.valid, err)
			}
		})
	}
}

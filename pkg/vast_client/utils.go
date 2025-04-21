package vast_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const ApplicationJson = "application/json"

// convertMapToQuery converts a map[string]any to a URL query string.
// Values are stringified using fmt.Sprint.
func convertMapToQuery(params Params) string {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, fmt.Sprint(v))
	}
	return values.Encode()
}

// getResponseBodyAsStr reads and returns the HTTP response body as a string.
// If the response body contains valid JSON, it returns a pretty-printed version.
// If the JSON indentation fails or the body is not JSON, it returns the raw body as a string.
// If the response is nil or an error occurs during reading, it returns an empty string.
//
// Note: This function consumes and closes the response body.
func getResponseBodyAsStr(r *http.Response) string {
	var b bytes.Buffer
	if r == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	//Let's try to make it a pretty json if not we will just dump the body
	err = json.Indent(&b, body, "", "  ")
	if err == nil {
		return string(b.Bytes())
	}
	return string(body)
}

// sanitizeVersion truncates all segments of Cluster Version above core (x.y.z)
func sanitizeVersion(version string) (string, bool) {
	segments := strings.Split(version, ".")
	truncated := len(segments) > 3
	return strings.Join(segments[:3], "."), truncated
}

func toInt(val any) (int64, error) {
	var idInt int64
	switch v := val.(type) {
	case int64:
		idInt = v
	case float64:
		idInt = int64(v)
	case int:
		idInt = int64(v)
	default:
		return 0, fmt.Errorf("unexpected type for id field: %T", v)
	}
	return idInt, nil
}

func toRecord(m map[string]interface{}) (Record, error) {
	converted := Record{}
	for k, v := range m {
		converted[k] = v
	}
	return converted, nil
}

func toRecordSet(list []map[string]any) (RecordSet, error) {
	records := make(RecordSet, 0, len(list))
	for _, item := range list {
		rec, err := toRecord(item)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// toStringIfInt Convert to string if val type is int
func toStringIfInt(val any) (string, error) {
	switch v := val.(type) {
	case int, float32, float64:
		return fmt.Sprintf("%v", v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

// toIntIfString converts string to int if possible, otherwise returns int as-is
func toIntIfString[T int | float64](val any) (T, error) {
	switch v := val.(type) {
	case float64:
		return T(v), nil
	case int:
		return T(v), nil
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string to int: %v", err)
		}
		return T(i), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// validateResponse checks the response for valid HTTP status codes (specifically for 2xx codes).
// It returns an error if the status code is not a valid 2xx code or if the response is nil.
//
// Arguments:
// - response: the HTTP response to validate
// - err: the error to check (if any)
//
// Returns:
// - response: the original HTTP response
// - error: an error if validation fails
func validateResponse(response *http.Response) (*http.Response, error) {
	// Check if the response status code is within the 2xx range (successful responses)
	if response == nil {
		return nil, errors.New("server unreachable: verify the host is correct and the network is accessible")
	}
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return response, nil
	}
	// If not, return an error indicating the invalid status code
	errStr := getResponseBodyAsStr(response)
	return response, fmt.Errorf("invalid status code %d, err: %s", response.StatusCode, errStr)
}

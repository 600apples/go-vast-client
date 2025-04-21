package vast_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bndr/gotabulate"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

const resourceTypeKey = "@resourceType"

var empty = struct{}{}
var printableAttrs = map[string]struct{}{
	"id":             empty,
	"name":           empty,
	"sys_version":    empty,
	"path":           empty,
	"tenant_id":      empty,
	"nqn":            empty,
	"ip_ranges":      empty,
	"volumes":        empty,
	"nguid":          empty,
	"subsystem_name": empty,
	"size":           empty,
	"block_host":     empty,
	"volume":         empty,
	"state":          empty,
}

//  ######################################################
//              FUNCTION PARAMS
//  ######################################################

// Params represents a generic set of key-value parameters,
// used for constructing query strings or request bodies.
type Params map[string]any

// ToQuery serializes the Params into a URL-encoded query string.
// This is useful for GET requests where parameters are passed via the URL.
func (pr *Params) ToQuery() string {
	return convertMapToQuery(*pr)
}

// ToBody serializes the Params into a JSON-encoded io.Reader,
// suitable for use as the body of an HTTP POST, PUT, or PATCH request.
func (pr *Params) ToBody() (io.Reader, error) {
	buffer, err := json.Marshal(*pr)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buffer), nil
}

// Update merges another Params map into the original Params.
// If a key already exists and `override` is true, its value is skipped.
// If a key doesn't exist, the key-value pair is added.
func (pr *Params) Update(other Params, override bool) {
	for key, value := range other {
		// If the key already exists in the original Params and override is false, skip it.
		if _, exists := (*pr)[key]; exists && override {
			continue
		}
		(*pr)[key] = value
	}
}

//  ######################################################
//              RETURN TYPES
//  ######################################################

// getPrintableAttrs returns a slice of keys to be printed from the Record
func getPrintableAttrs(r Record) []string {
	var attrs []string
	for key := range r {
		if _, ok := printableAttrs[key]; ok {
			attrs = append(attrs, key)
		}
	}
	sort.Strings(attrs) // Sort to keep consistent order
	return attrs
}

// Renderable is an interface implemented by types that can render themselves
// into a human-readable string format, typically for CLI display or logging.
type Renderable interface {
	Render() string
}

// Record represents a single generic data object as a key-value map.
// It's commonly used to unmarshal a single JSON object from an API response.
type Record map[string]any

// EmptyRecord represents a placeholder for methods that do not return data,
// such as DELETE operations. It maintains the same structure as Record
// but is used semantically to indicate the absence of returned content.
type EmptyRecord map[string]any

// RecordSet represents a list of Record objects.
// It is typically used to represent responses containing multiple items.
type RecordSet []Record

// RecordUnion defines a union of supported record types for generic operations.
// It can be a single Record, an EmptyRecord, or a RecordSet.
// This allows functions to operate on any supported response type
// using Go generics.
type RecordUnion interface {
	Record | EmptyRecord | RecordSet
}

// Fill populates the fields of the provided container struct using the values
// from the Record (a map[string]any). It uses reflection to match fields
// by their `json` tag names.
//
// The container must be a non-nil pointer to a struct. Each field with a `json`
// tag (excluding `-`) is matched to a corresponding key in the Record.
//
// Type conversions are attempted where necessary:
//   - If the field is a string and the value is an int, it will be converted using `strconv.Itoa`.
//   - If the field is an int (or int-like), and the value is a string, it will be parsed using `strconv.Atoi`.
//   - If the types are convertible via reflection, they will be converted accordingly.
//   - As a fallback, it attempts to marshal/unmarshal the value via JSON to fit the expected type.
//
// Fields that are not exported (i.e., unexported lowercase names) cannot be set
// and will cause an error if matched.
//
// Returns an error if the container is not a pointer to a struct or if a field
// cannot be set due to visibility or type incompatibility.
func (r *Record) Fill(container any) error {
	val := reflect.ValueOf(container)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("container must be a non-nil pointer to a struct")
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("container must point to a struct")
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		jsonKey := strings.Split(jsonTag, ",")[0]
		if !field.CanSet() {
			return fmt.Errorf("cannot set field %s. Make sure field has capitalized name", fieldType.Name)
		}

		if value, ok := (*r)[jsonKey]; ok {
			valToSet := reflect.ValueOf(value)

			if valToSet.Type().AssignableTo(field.Type()) {
				field.Set(valToSet)
			} else {
				// Custom int <-> string conversions
				switch field.Kind() {
				case reflect.String:
					strVal, err := toStringIfInt(value)
					if err == nil {
						field.SetString(strVal)
						continue
					}
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					switch v := value.(type) {
					case float64:
						if float64(int64(v)) == v {
							field.SetInt(int64(v))
							continue
						} else {
							field.SetFloat(v) // treat it as float, field must be float64 or this will panic
							continue
						}
					}
					if intVal, err := toIntIfString[int](value); err == nil {
						field.SetInt(int64(intVal))
						continue
					}
				default:
					if valToSet.Type().ConvertibleTo(field.Type()) {
						field.Set(valToSet.Convert(field.Type()))
						continue
					} else {
						// JSON fallback
						raw, err := json.Marshal(value)
						if err != nil {
							continue
						}
						newPtr := reflect.New(field.Type())
						if err := json.Unmarshal(raw, newPtr.Interface()); err != nil {
							continue
						}
						field.Set(newPtr.Elem())
					}
				}
			}
		}
	}
	return nil
}

// Render prints a single Record as a table
func (r Record) Render() string {
	headers := []string{"attr", "value"}
	var rows [][]any
	var name string
	if resourceTyp, ok := r[resourceTypeKey]; ok {
		name = resourceTyp.(string)
	} else {
		name = "<Unknown>"
	}
	if len(r) == 0 {
		return "<>"
	}
	// Iterate over printable attributes and add them to rows
	for _, key := range getPrintableAttrs(r) {
		if val, ok := r[key]; ok && val != nil {
			rows = append(rows, []any{key, fmt.Sprintf("%v", val)})
		}
	}

	// Collect remaining attributes that are not in printableAttrs
	remainingAttrs := make(map[string]any)
	for key, value := range r {
		if _, ok := printableAttrs[key]; !ok {
			if key == resourceTypeKey || value == nil {
				continue
			}
			remainingAttrs[key] = value
		}
	}
	if len(remainingAttrs) > 0 {
		// Marshal remainingAttrs into compact JSON
		remainingJSON, _ := json.Marshal(remainingAttrs)
		remainingJSONStr := string(remainingJSON)
		rows = append(rows, []any{"<<remaining attrs>>", remainingJSONStr})
	}
	t := gotabulate.Create(rows)
	t.SetHeaders(headers)
	t.SetAlign("left")
	t.SetWrapStrings(true)
	t.SetMaxCellSize(85)
	return fmt.Sprintf("%s:\n%s", name, t.Render("grid"))
}

// Render prints the full RecordSet by rendering each individual Record
func (rs RecordSet) Render() string {
	if len(rs) == 0 {
		return "[]"
	}
	var out strings.Builder
	out.WriteString("[\n")
	for i, record := range rs {
		out.WriteString(record.Render())
		if i < len(rs)-1 {
			out.WriteString("\n\n") // separate entries with a blank line
		}
	}
	out.WriteString("\n]")
	return out.String()
}

// Render EmptyRecord
func (er EmptyRecord) Render() string {
	return "<>"
}

// unmarshalToRecordUnion unmarshall the response body into a generic Record/RecordSet structure.
func unmarshalToRecordUnion[T RecordUnion](
	response *http.Response,
) (T, error) {
	var result T

	switch any(result).(type) {
	case EmptyRecord:
		return result, nil
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// applyCallbackForRecordUnion applies the provided callback function to a response if
// the response type matches the specified generic type T. It supports different types
// of Renderable responses (Record, RecordSet, and EmptyRecord), and will only apply the
// callback for the exact type matching the generic type T.
func applyCallbackForRecordUnion[T RecordUnion](response Renderable, callback func(Renderable) (Renderable, error)) (Renderable, error) {
	switch typed := response.(type) {
	case Record:
		var zero T
		if _, ok := any(zero).(Record); ok {
			return callback(typed)
		}
		return typed, nil

	case RecordSet:
		var zero T
		if _, ok := any(zero).(RecordSet); ok {
			return callback(typed)
		}
		return typed, nil

	case EmptyRecord:
		var zero T
		if _, ok := any(zero).(EmptyRecord); ok {
			return callback(typed)
		}
		return typed, nil

	default:
		return nil, fmt.Errorf("unsupported type %T for result", response)
	}
}

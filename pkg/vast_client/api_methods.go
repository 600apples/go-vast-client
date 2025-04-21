package vast_client

import (
	"context"
	"errors"
	"fmt"
	version "github.com/hashicorp/go-version"
	"net/http"
)

//  ######################################################
//              VAST RESOURCES BASE CRUD OPS
//  ######################################################

type NotFoundError struct {
	Resource string
	Query    string
}

func isNotFoundErr(err error) bool {
	var nfErr *NotFoundError
	if errors.As(err, &nfErr) {
		return true
	}
	return false
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource '%s' not found for params '%s'", e.Resource, e.Query)
}

// VastResource defines the interface for standard CRUD operations on a VAST resource.
type VastResource interface {
	Session() RESTSession
	GetResourceType() string
	List(context.Context, Params) (RecordSet, error)
	Create(context.Context, Params) (Record, error)
	Update(context.Context, int64, Params) (Record, error)
	Delete(context.Context, Params) (EmptyRecord, error)
	Ensure(context.Context, string, Params) (Record, error)
	DeleteById(context.Context, int64) (EmptyRecord, error)
	Get(context.Context, Params) (Record, error)
	GetById(context.Context, int64) (Record, error)
}

// InterceptableVastResource combines request interception with vast resource behavior.
type InterceptableVastResource interface {
	RequestInterceptor
	VastResource
}

func setResourceKey[T RecordUnion](result T, err error, resourceType string) (T, error) {
	// Set resource type key for tabular formatting only if not already set.
	if err != nil {
		return result, err
	}
	switch v := any(result).(type) {
	case Record:
		if _, ok := v[resourceTypeKey]; !ok {
			v[resourceTypeKey] = resourceType
		}
		return any(v).(T), nil
	case RecordSet:
		for _, rec := range v {
			if _, ok := rec[resourceTypeKey]; !ok {
				rec[resourceTypeKey] = resourceType
			}
		}
		return any(v).(T), nil
	case EmptyRecord:
		return any(v).(T), nil
	default:
		return result, fmt.Errorf("unsupported type")
	}
}

// Check if current VAST cluster version support triggered API
func checkVastResourceVersionCompat(ctx context.Context, e *VastResourceEntry) error {
	if e.availableFromVersion == nil {
		return nil
	}
	compareOrd, err := e.rest.Versions.CompareWith(ctx, e.availableFromVersion)
	if err != nil {
		return err
	}
	clusterVersion, _ := e.rest.Versions.GetVersion(ctx)
	if compareOrd == -1 {
		return fmt.Errorf("resource %q is not supported in VAST cluster version %s (supported from version %s)", e.resourceType, clusterVersion, e.availableFromVersion)
	}
	return nil
}

// VastResourceEntry implements VastResource and provides common behavior for managing VAST resources.
type VastResourceEntry struct {
	resourcePath         string
	resourceType         string
	apiVersion           string
	availableFromVersion *version.Version
	rest                 *VMSRest
}

// Session returns the current VMSSession associated with the resource.
func (e *VastResourceEntry) Session() RESTSession {
	return e.rest.Session
}

func (e *VastResourceEntry) GetResourceType() string {
	return e.resourceType
}

// List retrieves all resources matching the given parameters.
func (e *VastResourceEntry) List(ctx context.Context, params Params) (RecordSet, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	return request[RecordSet](ctx, e, http.MethodGet, e.resourcePath, e.apiVersion, params, nil)
}

// Create creates a new resource using the provided parameters.
func (e *VastResourceEntry) Create(ctx context.Context, body Params) (Record, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	return request[Record](ctx, e, http.MethodPost, e.resourcePath, e.apiVersion, nil, body)
}

// Update updates an existing resource by its ID using the provided parameters.
func (e *VastResourceEntry) Update(ctx context.Context, id int64, body Params) (Record, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/%d", e.resourcePath, id)
	return request[Record](ctx, e, http.MethodPatch, path, e.apiVersion, nil, body)
}

// Delete finds and deletes a resource using the provided query and body parameters.
func (e *VastResourceEntry) Delete(ctx context.Context, params Params) (EmptyRecord, error) {
	result, err := e.Get(ctx, params)
	if err != nil {
		if isNotFoundErr(err) {
			// Resource not found. For "Delete" it is not error condition.
			// If you want custom logic you can implement your own Get logic and then ue "DeleteById"
			return EmptyRecord{}, nil
		}
		return nil, err
	}
	idVal, ok := result["id"]
	if !ok {
		return nil, fmt.Errorf("resource '%s' does not have id field in body and thereby cannot be deleted by id")
	}
	idInt, err := toInt(idVal)
	if err != nil {
		return nil, err
	}
	return e.DeleteById(ctx, idInt)
}

// DeleteById deletes a resource using its unique ID.
func (e *VastResourceEntry) DeleteById(ctx context.Context, id int64) (EmptyRecord, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/%d", e.resourcePath, id)
	return request[EmptyRecord](ctx, e, http.MethodDelete, path, e.apiVersion, nil, nil)
}

// Ensure checks if a resource with the given name exists, and creates it if not.
func (e *VastResourceEntry) Ensure(ctx context.Context, name string, body Params) (Record, error) {
	result, err := e.Get(ctx, Params{"name": name})
	if isNotFoundErr(err) {
		body["name"] = name
		return e.Create(ctx, body)
	} else if err != nil {
		return nil, err
	}
	return result, nil
}

// Get retrieves a single resource based on the given parameters. Returns NotFoundError if no resource matches.
func (e *VastResourceEntry) Get(ctx context.Context, params Params) (Record, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	result, err := request[RecordSet](ctx, e, http.MethodGet, e.resourcePath, e.apiVersion, params, nil)
	if err != nil {
		return nil, err
	}
	switch len(result) {
	case 0:
		return nil, &NotFoundError{
			Resource: e.resourcePath,
			Query:    params.ToQuery(),
		}
	case 1:
		return result[0], nil
	default:
		return nil, fmt.Errorf("more than one resource '%s' found for params '%v'", e.resourcePath, params.ToQuery())
	}
}

// GetById retrieves a resource by its unique ID.
func (e *VastResourceEntry) GetById(ctx context.Context, id int64) (Record, error) {
	if err := checkVastResourceVersionCompat(ctx, e); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/%d", e.resourcePath, id)
	return request[Record](ctx, e, http.MethodGet, path, e.apiVersion, nil, nil)
}

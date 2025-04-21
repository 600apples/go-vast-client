package vast_client

import (
	"context"
	"fmt"
	version "github.com/hashicorp/go-version"
	"net/http"
	"strings"
	"time"
)

//  ######################################################
//              FINAL VAST RESOURCES
//  ######################################################

type VastResourceType interface {
	Version |
	Quota |
	View |
	VipPool |
	User |
	UserKey |
	Snapshot |
	BlockHost |
	Volume |
	VTask |
	BlockHostMapping |
	Cnode |
	QosPolicy |
	Dns |
	ViewPolicy |
	Group |
	Nis |
	Tenant |
	Ldap |
	S3LifeCycleRule |
	ActiveDirectory |
	S3Policy |
	ProtectedPath |
	GlobalSnapshotStream |
	ReplicationPeers |
	ProtectionPolicy |
	S3replicationPeers |
	Realm |
	Role
}

// ------------------------------------------------------

type Version struct {
	*VastResourceEntry
}

var sysVersion *version.Version

func (v *Version) GetVersion(ctx context.Context) (*version.Version, error) {
	if sysVersion != nil {
		return sysVersion, nil
	}
	result, err := v.List(ctx, Params{"status": "success"})
	if err != nil {
		return nil, err
	}
	truncatedVersion, _ := sanitizeVersion(result[0]["sys_version"].(string))
	clusterVersion, err := version.NewVersion(truncatedVersion)
	if err != nil {
		return nil, err
	}
	//We only work with core version
	sysVersion = clusterVersion.Core()
	return sysVersion, nil
}

func (v *Version) CompareWith(ctx context.Context, other *version.Version) (int, error) {
	clusterVersion, err := v.GetVersion(ctx)
	if err != nil {
		return 0, err
	}
	return clusterVersion.Compare(other), nil
}

// ------------------------------------------------------

type Quota struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type View struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type VipPool struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type User struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type UserKey struct {
	*VastResourceEntry
}

func (uk *UserKey) CreateKey(ctx context.Context, userId int64) (Record, error) {
	path := fmt.Sprintf(uk.resourcePath, userId)
	return request[Record](ctx, uk, http.MethodPost, path, uk.apiVersion, nil, nil)
}

func (uk *UserKey) DeleteKey(ctx context.Context, userId int64, accessKey string) (EmptyRecord, error) {
	path := fmt.Sprintf(uk.resourcePath, userId)
	return request[EmptyRecord](ctx, uk, http.MethodDelete, path, uk.apiVersion, nil, Params{"access_key": accessKey})
}

// ------------------------------------------------------

type Cnode struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type QosPolicy struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Dns struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type ViewPolicy struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Group struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Nis struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Tenant struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Ldap struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type S3LifeCycleRule struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type ActiveDirectory struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type S3Policy struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type ProtectedPath struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type GlobalSnapshotStream struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type ReplicationPeers struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type ProtectionPolicy struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type S3replicationPeers struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Realm struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Role struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type Snapshot struct {
	*VastResourceEntry
}

func (s *Snapshot) afterRequest(response Renderable) (Renderable, error) {
	// List of snapshots is returned under "results" key
	return applyCallbackForRecordUnion[RecordSet](response, func(r Renderable) (Renderable, error) {
		// This callback is only invoked if response is a RecordSet
		if rawMap, ok := any(r).(map[string]interface{}); ok {
			if inner, found := rawMap["results"]; found {
				if list, ok := inner.([]map[string]any); ok {
					return toRecordSet(list)
				}
			}
		}
		return r, nil
	})
}

// ------------------------------------------------------

type BlockHost struct {
	*VastResourceEntry
}

func (bh *BlockHost) EnsureBlockHost(ctx context.Context, name string, tenantId int, nqn string) (Record, error) {
	params := Params{"name": name, "tenant_id": tenantId}
	blockHost, err := bh.Get(ctx, params)
	if isNotFoundErr(err) {
		params.Update(Params{"nqn": nqn, "os_type": "LINUX", "connectivity_type": "tcp"}, false)
		return bh.Create(ctx, params)
	} else if err != nil {
		return nil, err
	}
	return blockHost, nil
}

// ------------------------------------------------------

type Volume struct {
	*VastResourceEntry
}

// ------------------------------------------------------

type VTask struct {
	*VastResourceEntry
}

// WaitTask waits for the task to complete
func (t *VTask) WaitTask(ctx context.Context, taskId int64) (Record, error) {
	// isTaskComplete checks if the task is complete
	isTaskComplete := func(taskId int64) (Record, error) {
		task, err := t.GetById(ctx, taskId)
		if err != nil {
			return nil, err
		}
		// Check the task state
		taskName := fmt.Sprintf("%v", task["name"])
		taskState := strings.ToLower(fmt.Sprintf("%v", task["state"]))
		_taskId, err := toInt(task["id"])
		if err != nil {
			return nil, err
		}
		switch taskState {
		case "completed":
			return task, nil
		case "running":
			return nil, fmt.Errorf("task %s with ID %s is still running, timeout occurred", taskName, _taskId)
		default:
			rawMessages := task["messages"]
			messages, ok := rawMessages.([]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected message format: %T", rawMessages)
			}
			if len(messages) == 0 {
				return nil, fmt.Errorf("task %s failed with ID %s: no messages found", taskName, _taskId)
			}
			lastMsg := fmt.Sprintf("%v", messages[len(messages)-1])
			return nil, fmt.Errorf("task %s failed with ID %s: %s", taskName, _taskId, lastMsg)
		}
	}
	// Retry logic to poll the task status
	retries := 30
	interval := time.Millisecond * 500
	backoffRate := 1

	for retries > 0 {
		task, err := isTaskComplete(taskId)
		if err == nil {
			return task, nil
		}
		time.Sleep(interval)
		// Backoff logic
		interval *= time.Duration(backoffRate)
		retries--
	}
	return nil, fmt.Errorf("task did not complete in time")
}

// ------------------------------------------------------

type BlockHostMapping struct {
	*VastResourceEntry
}

func (bhm *BlockHostMapping) Map(ctx context.Context, hostId, volumeId int64) (Record, error) {
	body := Params{
		"pairs_to_add": []Params{
			{
				"host_id":   hostId,
				"volume_id": volumeId,
			},
		},
	}
	path := fmt.Sprintf("%s/bulk", bhm.resourcePath)
	// Make request on behalf of VTask (for proper parsing)
	task, err := request[Record](ctx, bhm, http.MethodPatch, path, bhm.apiVersion, nil, body)
	if err != nil {
		return nil, err
	}
	intVal, err := toInt(task["id"])
	if err != nil {
		return nil, err
	}
	return bhm.rest.VTasks.WaitTask(ctx, intVal)
}

func (bhm *BlockHostMapping) UnMap(ctx context.Context, hostId, volumeId int64) (Record, error) {
	body := Params{
		"pairs_to_remove": []Params{
			{
				"host_id":   hostId,
				"volume_id": volumeId,
			},
		},
	}
	path := fmt.Sprintf("%s/bulk", bhm.resourcePath)
	task, err := request[Record](ctx, bhm, http.MethodPatch, path, bhm.apiVersion, nil, body)
	if err != nil {
		return nil, err
	}
	intVal, err := toInt(task["id"])
	if err != nil {
		return nil, err
	}
	return bhm.rest.VTasks.WaitTask(ctx, intVal)
}

func (bhm *BlockHostMapping) EnsureMap(ctx context.Context, hostId, volumeId int64) (Record, error) {
	result, err := bhm.Get(ctx, Params{"volume__id": volumeId, "block_host__id": hostId})
	if isNotFoundErr(err) {
		return bhm.Map(ctx, hostId, volumeId)
	}
	return result, err
}

# GO Vast client


[![License: MIT](https://img.shields.io/badge/License-Apache2-yellow.svg)](https://opensource.org/licenses/MIT)

The Vast Go client provides a convenient and typed interface for interacting with the VAST Data REST API.
It wraps low-level HTTP operations with structured methods and helpers, enabling you to manage different resources.


### VMSConfig: Client Configuration

The VMSConfig struct defines how the Vast Go client connects to the VMS API server.

Configuration example:
```go
import (
    "time"
    "vast_client/client"
)

func main() {
    timeout := 30 * time.Second
    config := &client.VMSConfig{
        Host:           "10.27.40.1",
        Port:           443,
        Username:       "admin",
        Password:       "123456",
        SslVerify:      true,
        Timeout:        &timeout,
        MaxConnections: 10,
        UserAgent:      "vast-go-client/1.0",
        ApiVersion:     "v5",
        BeforeRequestFn: func(ctx context.Context, verb, url string, body io.Reader) error {
            // Example of BeforeRequest interceptor.
            // Interceptor takes copy of body so you can read from it safely.
            log.Printf("Sending request: verb=%s, url=%s", verb, url)
            if body != nil {
               bodyBytes, err := io.ReadAll(body)
                if err != nil { return err }
                var pretty bytes.Buffer
                if err = json.Indent(&pretty, bodyBytes, "", "  "); err == nil {
                    log.Printf("Request JSON:\n%s", pretty.String())
                } else {
                    log.Printf("Request Body:\n%s", string(bodyBytes))
                }
            }
            return nil
        },
        AfterRequestFn: func(response client.Renderable) (client.Renderable, error) {
            // Example of AfterRequest interceptor.
            log.Printf("Result:\n%s", response.Render())
            return response, nil
        },
    }
    rest := client.NewVMSRest(config)
}
```

### Configuration Parameters

| Field           | Type       | Description                                                                        | Required | Default |
|-----------------|------------|------------------------------------------------------------------------------------|--------|----|
| `Host`          | `string`   | Hostname or IP of the VMS API server.                                              | ✅      | —  |
| `Port`          | `uint64`   | Port for the API server.                                                           | ❌      | `443` |
| `Username`      | `string`   | Username for basic auth (used with `Password`).                                    | ⚠️     | —  |
| `Password`      | `string`   | Password for basic auth (used with `Username`).                                    | ⚠️     | —  |
| `ApiToken`      | `string`   | Optional bearer token (alternative to username/password).                          | ⚠️     | —  |
| `SslVerify`     | `bool`     | Verify SSL certificates when `true`.                                               | ❌      | `false` |
| `Timeout`       | `*time.Duration` | HTTP timeout for API requests. If `nil`, a default is used.                        | ❌      | `30s` |
| `MaxConnections`| `int`      | Max concurrent HTTP connections.                                                   | ❌      | `10` |
| `UserAgent`     | `string`   | Optional custom `User-Agent` string for HTTP requests.                             | ❌      | `vast-go-client` |
| `BeforeRequestFn`    | `func(ctx context.Context, verb, url string, body io.Reader) error` | Optional hook executed before each request. Useful for logging or mutation.        | ❌      | —  |
| `AfterRequestFn`    | `func(response Renderable) (Renderable, error)` | Optional hook executed after receiving a response. Useful for logging or mutation. | ❌   | —  |


### VMSRest: Entry Point to VAST API Resources

The `VMSRest` object serves as the primary interface to interact with the VAST Data API.
It acts as a container for multiple subresources, each representing a logical component of the VAST system (e.g., views, volumes, snapshots).

You typically initialize it like so:

```go
ctx := context.Background()
config := &client.VMSConfig{
    Host:     "10.27.40.1",
    Username: "admin",
    Password: "123456",
}

rest := client.NewVMSRest(config)
```

Subresources
The VMSRest object includes multiple subresources (e.g., rest.Views, rest.Quotas, rest.Volumes, etc.).

Each subresource has the following standard methods:

- List
- Get
- GetById
- Create
- Update
- Ensure
- Delete
- DeleteById

!!! note
    Additionally, a resource can define extra methods to handle "non-standard" URLs, such as endpoints that return asynchronous tasks or perform custom operations.

Examples:
```go
// Create a volume
result, err := rest.Volumes.Create(ctx, client.Params{"name": "myvolume", "size": 10 * 512, "view_id": 3})

fmt.Println("Name -> ", result["name"])
fmt.Println("Uuid -> ", result["uuid"])
fmt.Println(result.Render())
```

```go
// Ensure view (Get by name or Create with provided name and additional params):
params := client.Params{"path": "/myblock", "protocols": []string{"BLOCK"}, "policy_id": 1}
result, err := rest.Views.Ensure(ctx, "myview", params)

fmt.Println("Name -> ", result["name"])
fmt.Println("Protocols -> ", result["protocols"])
fmt.Println("Tenant -> ", result["tenant_name"])
fmt.Println("QosPolicy -> ", result["qos_policy"])
fmt.Println(result.Render())
```

```go
// Get Vippool
result, err := rest.VipPools.Get(ctx, client.Params{"name": "vippool-1", "tenant_id": 1})

fmt.Println("Name -> ", result["name"])
fmt.Println("StartIp -> ", result["start_ip"])
fmt.Println("EndIp -> ", result["end_ip"])
fmt.Println(result.Render())
```

```go
// Delete Quota  (Get quota by search params and if found delete it. Not found is not error condition)
_, err = rest.Quotas.Delete(ctx, client.Params{"path__endswith": "foobar"})
```

```go
// Delete Quota by ID
_, err = rest.Quotas.DeleteById(ctx, 25)
```

### Working with Record: .Render() and .Fill()

Pretty Printing: The Record type includes a `.Render` method for printing data in a readable tabular format.

#### Render

```go
fmt.Println(result.Render())

VipPool:
+------------------------+--------------------------------------+
| attr                   | value                                |
+========================+======================================+
| id                     | 2                                    |
+------------------------+--------------------------------------+
| ip_ranges              | [[10.0.0.1 10.0.0.16]]               |
+------------------------+------------------------------ -------+
| name                   | vippool-1                            |
..................
```

#### Fill

You can define a Go struct with matching fields and JSON tags to map the API response:
```go
type ViewContainer struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	TenantID int64  `json:"tenant_id"`
}
```

Then use `.Fill` to populate struct from the API result:
```go
ctx := context.Background()
config := &client.VMSConfig{
    Host:     "10.27.40.1",
    Username: "admin",
    Password: "123456",
}

rest := client.NewVMSRest(config)

result, err := rest.Views.Ensure(ctx, "myvolume", client.Params{
    "path":      "/myblock",
    "protocols": []string{"BLOCK"},
    "policy_id": 1,
})
if err != nil {
    log.Fatal(err)
}

var view ViewContainer
if err := result.Fill(&view); err != nil {  // <- Make sure you passed pointer here
    log.Fatal(err)
}

fmt.Println("View name:", view.Name)
fmt.Println("Path:", view.Path)
fmt.Println("ID:", view.ID)
fmt.Println("Tenant ID:", view.TenantID)
```

!!! note
    The struct must have valid json tags for .Fill() to work correctly.


### Low level Client API methods

Subresources are being gradually integrated into the `VMSRest` object.
If a specific resource is not yet available, you can use the lower-level Client API methods as a fallback.

Rest Session implements 5 methods
```go
Get(context.Context, string, io.Reader) (*http.Response, error)
Post(context.Context, string, io.Reader) (*http.Response, error)
Put(context.Context, string, io.Reader) (*http.Response, error)
Patch(context.Context, string, io.Reader) (*http.Response, error)
Delete(context.Context, string, io.Reader) (*http.Response, error)
```

Example:

```go
ctx := context.Background()
config := &client.VMSConfig{
    Host:     "10.27.40.1",
    Username: "admin",
    Password: "123456",
}

rest := client.NewVMSRest(config)

path := "views"
query := "name=MyView"
apiVer := "v5"

//  Helper to build full url (host/port are taken from config)
url, err := rest.BuildUrl(path, query, apiVer)
if err != nil {
    log.Fatal(err)
}

response, err := rest.Session.Get(ctx, url, nil)
if err != nil {
    log.Fatal(err)
}
body, err := io.ReadAll(response.Body)
if err != nil {
    log.Fatal(err)
}
defer response.Body.Close()
result := []map[string]any{}
err = json.Unmarshal(body, &result)
```

!!! note
    "Low level" methods return *http.Response so you need to read from response and parse it.

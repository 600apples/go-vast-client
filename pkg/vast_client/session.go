package vast_client

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
)

type RESTSession interface {
	Get(context.Context, string, io.Reader) (*http.Response, error)
	Post(context.Context, string, io.Reader) (*http.Response, error)
	Put(context.Context, string, io.Reader) (*http.Response, error)
	Patch(context.Context, string, io.Reader) (*http.Response, error)
	Delete(context.Context, string, io.Reader) (*http.Response, error)
	GetConfig() *VMSConfig
	sync.Locker
}

type VMSSession struct {
	config *VMSConfig
	client *http.Client
	mu     sync.Mutex
	auth   Authenticator
}

type VMSSessionMethod func(context.Context, string, io.Reader) (*http.Response, error)

func NewVMSSession(config *VMSConfig) *VMSSession {
	//Create a new session object
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !config.SslVerify}
	transport.MaxConnsPerHost = config.MaxConnections
	transport.IdleConnTimeout = *config.Timeout
	client := &http.Client{Transport: transport}
	return &VMSSession{
		config: config,
		client: client,
		auth:   CreateAuthenticator(config),
	}
}

func request[T RecordUnion](
	ctx context.Context,
	r InterceptableVastResource,
	verb, path, apiVer string,
	params, body Params,
) (T, error) {
	var (
		vmsMethod           VMSSessionMethod
		query               string
		data                io.Reader
		beforeRequestCbData io.Reader
		err                 error
	)
	verb = strings.ToUpper(verb)
	session := r.Session()

	switch verb {
	case "GET":
		vmsMethod = session.Get
	case "POST":
		vmsMethod = session.Post
	case "PUT":
		vmsMethod = session.Put
	case "PATCH":
		vmsMethod = session.Patch
	case "DELETE":
		vmsMethod = session.Delete
	default:
		return nil, fmt.Errorf("unknown verb: %s", verb)
	}
	if params != nil {
		query = params.ToQuery()
	}
	if body != nil {
		data, err = body.ToBody()
		if err != nil {
			return nil, err
		}
		// Need to copy of dta for BeforeRequest Interceptor
		beforeRequestCbData, err = body.ToBody()
		if err != nil {
			return nil, err
		}
	} else {
		data = bytes.NewReader(nil)
	}
	url, err := buildUrl(session, path, query, apiVer)
	if err != nil {
		return nil, err
	}
	// before request interceptor
	if err = r.doBeforeRequest(ctx, verb, url, beforeRequestCbData); err != nil {
		return nil, err
	}
	response, err := vmsMethod(ctx, url, data)
	if err != nil {
		return nil, err
	}
	result, err := unmarshalToRecordUnion[T](response)
	if err != nil {
		fmt.Println(err)
	}
	// Set resource type key so .Render can recognize resource type
	result, err = setResourceKey[T](result, err, r.GetResourceType())
	if err != nil {
		return nil, err
	}
	// after request interceptor
	interceptedResult, err := r.doAfterRequest(Renderable(result))
	if err != nil {
		return nil, err
	}
	return interceptedResult.(T), nil
}

func (s *VMSSession) Get(ctx context.Context, url string, _ io.Reader) (*http.Response, error) {
	return doRequest(ctx, s, http.MethodGet, url, nil)
}

func (s *VMSSession) Post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return doRequest(ctx, s, http.MethodPost, url, body)
}

func (s *VMSSession) Put(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return doRequest(ctx, s, http.MethodPut, url, body)
}

func (s *VMSSession) Patch(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return doRequest(ctx, s, http.MethodPatch, url, body)
}

func (s *VMSSession) Delete(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return doRequest(ctx, s, http.MethodDelete, url, body)
}

func (s *VMSSession) GetConfig() *VMSConfig {
	return s.config
}
func (s *VMSSession) Lock()   { s.mu.Lock() }
func (s *VMSSession) Unlock() { s.mu.Unlock() }

func setupHeaders(s *VMSSession, r *http.Request) error {
	if err := s.auth.SetAuthHeader(s, &r.Header); err != nil {
		return err
	}
	r.Header.Add("Accept", ApplicationJson)
	r.Header.Add("Content-type", ApplicationJson)
	userAgent := fmt.Sprintf("%s, OS:%s, Arch:%s", s.config.UserAgent, runtime.GOOS, runtime.GOARCH)
	r.Header.Set("User-Agent", userAgent)
	return nil
}

func doRequest(ctx context.Context, s *VMSSession, verb, url string, body io.Reader) (*http.Response, error) {
	// Create the new HTTP request using the context
	if body == nil {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, verb, url, body)
	if err != nil {
		return nil, fmt.Errorf("request failed with error: %w", err)
	}
	if setHeadersErr := setupHeaders(s, req); setHeadersErr != nil {
		return nil, setHeadersErr
	}
	response, responseErr := s.client.Do(req)
	if responseErr != nil {
		return nil, fmt.Errorf("failed to perform %s request to %s, error %v", verb, url, responseErr)
	}
	return validateResponse(response)
}

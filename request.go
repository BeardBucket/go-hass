package hass

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"time"
)

const PathTypeAPI = "API"

// Doer represents an http client that can "Do" a request
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Access is the access and credentials for the API
type Access struct {
	host        string
	password    string
	token       string
	bearertoken string
	pathMap     map[string]string
	client      Doer
}

// NewAccess returns a new *Access to be used to interface with the
// Home Assistant system.
func NewAccess(host, password string) *Access {
	a := Access{
		host:     host,
		password: password,
		pathMap:  make(map[string]string),
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
	a.SetPath(PathTypeAPI, "/api") // Set default api path
	return &a
}

// SetPath sets the base api path to prepend to all requests of the given type
func (a *Access) SetPath(pType, path string) {
	a.pathMap[pType] = path
}

// GetPath gets the base api path to prepend to all requests of the given type
func (a *Access) GetPath(pType string) string {
	val, ok := a.pathMap[pType]
	if ok {
		return val
	} else {
		return ""
	}
}

// BuildURL creates a URL for requests
func (a *Access) BuildURL(pType, pth string) (string, error) {
	base := a.GetPath(pType)

	// Deconstruct the host url
	u, err := url.Parse(a.host)
	if err != nil {
		return "", err
	}

	// Update path with any path passed in via URL combined with the base for the type
	// and the passed in path
	u.Path = path.Join(u.Path, base, pth)

	return u.String(), nil
}

// SetAccess changes login credentials for API access
func (a *Access) SetAccess(host, password string) {
	a.host = host
	a.password = password
}

// SetClient allows you to specify a different http client than the default
func (a *Access) SetClient(client Doer) {
	a.client = client
}

// SetToken sets the X-HASSIO-KEY header
func (a *Access) SetToken(token string) {
	a.token = token
}

// SetBearerToken sets the Authentiation: Bearer header
// Long Lived Access Tokens can be generated from the HASS UI
func (a *Access) SetBearerToken(token string) {
	a.bearertoken = "Bearer " + token
}

func (a *Access) httpGet(pType, path string, v interface{}) error {
	url, err := a.BuildURL(pType, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	a.authorizeRequest(req)

	success := false
	for i := 0; i < 3; i++ { // Retry three times
		func() {
			var resp *http.Response
			resp, err = a.client.Do(req)
			if err != nil {
				return
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				err = errors.New("hass: status not OK: " + resp.Status)
				return
			}

			dec := json.NewDecoder(resp.Body)
			err = dec.Decode(v)
			success = true
		}()

		if success {
			break
		}
	}

	return err
}

func (a *Access) httpPost(pType, path string, v interface{}) error {
	var req *http.Request

	url, err := a.BuildURL(pType, path)
	if err != nil {
		return err
	}

	if v != nil {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}

		req, err = http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
	} else {
		var err error
		req, err = http.NewRequest("POST", url, nil)
		if err != nil {
			return err
		}
	}

	a.authorizeRequest(req)

	success := false
	for i := 0; i < 3; i++ {
		func() {
			var resp *http.Response
			resp, err = a.client.Do(req)
			if err != nil {
				return
			}

			defer resp.Body.Close()

			success = true
		}()

		if success {
			break
		}
	}

	return err
}

func (a *Access) authorizeRequest(req *http.Request) {
	if a.password != "" {
		req.Header.Set("x-ha-access", a.password)
	}

	if a.token != "" {
		req.Header.Set("X-HASSIO-KEY", a.token)
	}

	if a.bearertoken != "" {
		req.Header.Set("Authorization", a.bearertoken)
	}
}

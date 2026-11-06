package api_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type ApiClientResponse struct {
	Body       []byte
	Header     http.Header
	StatusCode int
	Status     string
}
type ApiClientResponseError struct {
	StatusCode int
	Timestamp  string
	Status     string
	Path       string
	Ctx        *map[string]any
	ErrorId    *string
	Errors     map[string]ApiClientErrorError
	Message    *string
}

type ApiClientErrorError struct {
	Messages []string
	Value    any
}

func populateHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if req.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
}

func populateSecurity(req *http.Request, c *ApiClient) {
	const (
		authKey      = "Authorization"
		apiKeyPrefix = "ApiKey"
	)
	if req == nil || c == nil {
		return
	}
	apiKey := c.configuration.apiKey
	req.Header.Add(authKey, fmt.Sprintf("%s %s", apiKeyPrefix, apiKey))
}

// not sure if we should use body io.Reader or any like in the _poly
func (c *ApiClient) call(ctx context.Context, method string, uri string, body io.Reader) (*ApiClientResponse, error) {
	fullUrl, err := url.JoinPath(c.configuration.serverUrl, uri)
	if err != nil {
		tflog.Debug(ctx, "error joining path", map[string]interface{}{
			"error":     err,
			"uri":       uri,
			"serverUrl": c.configuration.serverUrl,
			"fullUrl":   fullUrl,
		})
		return nil, err
	}

	req, err := http.NewRequest(method, fullUrl, body)
	if err != nil {
		tflog.Debug(ctx, "error creating request", map[string]interface{}{
			"error":     err,
			"uri":       uri,
			"serverUrl": c.configuration.serverUrl,
			"fullUrl":   fullUrl,
		})
		return nil, err
	}
	populateHeaders(req)
	populateSecurity(req, c)

	res, err := c.client.Do(req)
	if err != nil || res == nil {
		tflog.Debug(ctx, "error sending request", map[string]interface{}{
			"url":    fullUrl,
			"method": method,
			"error":  err,
			"res":    res,
		})
		if err != nil {
			err = fmt.Errorf("error sending request: %w", err)
		} else {
			err = fmt.Errorf("error sending request: no response")
		}
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	apiClientResponse := &ApiClientResponse{
		Body:       respBody,
		Header:     res.Header,
		StatusCode: res.StatusCode,
		Status:     res.Status,
	}
	return apiClientResponse, ResponseError(apiClientResponse)
}

func ResponseError(res *ApiClientResponse) error {
	// hardocded, not the best
	if res.StatusCode == 200 || res.StatusCode == 201 {
		return nil
	}
	var errorArr []error
	var apiClientResponseError ApiClientResponseError
	err := json.Unmarshal(res.Body, &apiClientResponseError)
	if err == nil {
		if apiClientResponseError.Message != nil {
			errorArr = append(errorArr, fmt.Errorf("response error: %s", *apiClientResponseError.Message))
		}
		for _, e := range apiClientResponseError.Errors {
			for _, message := range e.Messages {
				errorArr = append(errorArr, errors.New(message))
			}
		}
	} else {
		errorArr = append(errorArr, fmt.Errorf("response error: %s", res.Status))
	}
	return errors.Join(errorArr...)
}

// TODO: Lignes ci-dessous à virer si on utilise le polymorphic fetcher
func (c *ApiClient) Options(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodOptions, uri, body)
}

func (c *ApiClient) Get(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodGet, uri, body)
}

func (c *ApiClient) Head(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodHead, uri, body)
}

func (c *ApiClient) Post(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodPost, uri, body)
}

func (c *ApiClient) Put(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodPut, uri, body)
}

func (c *ApiClient) Delete(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodDelete, uri, body)
}

func (c *ApiClient) Trace(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodTrace, uri, body)
}

func (c *ApiClient) Connect(ctx context.Context, uri string, body io.Reader) (*ApiClientResponse, error) {
	return c.call(ctx, http.MethodConnect, uri, body)
}

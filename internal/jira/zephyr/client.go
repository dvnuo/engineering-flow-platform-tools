package zephyr

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jira"
)

type Client struct {
	JiraClient *httpclient.Client
	BasePath   string
	APIFamily  string
}

func NewClient(ctx *jira.Context, cfg EffectiveConfig) *Client {
	return &Client{JiraClient: ctx.Client, BasePath: cfg.RESTPath, APIFamily: cfg.APIFamily}
}

func (c *Client) ZAPI(p string) string {
	return ZAPI(c.BasePath, p)
}

func (c *Client) RawPath(p string) (string, error) {
	return RawPath(c.BasePath, p)
}

func (c *Client) Get(p string, q map[string]string) (interface{}, error) {
	return c.DoJSON(http.MethodGet, c.ZAPI(p), q, nil)
}

func (c *Client) Post(p string, body interface{}) (interface{}, error) {
	return c.DoJSON(http.MethodPost, c.ZAPI(p), nil, body)
}

func (c *Client) Put(p string, body interface{}) (interface{}, error) {
	return c.DoJSON(http.MethodPut, c.ZAPI(p), nil, body)
}

func (c *Client) DoJSON(method, p string, q map[string]string, body interface{}) (interface{}, error) {
	resp, err := c.JiraClient.Do(httpclient.Request{Method: method, Path: p, Query: q, JSONBody: body})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ReadJSONValue(resp.Body)
}

func ReadJSONValue(r io.Reader) (interface{}, error) {
	var out interface{}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(b)) == "" {
		return map[string]interface{}{"ok": true}, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]interface{}{"raw": string(b)}, nil
	}
	return out, nil
}

func PathEscape(s string) string {
	return url.PathEscape(s)
}

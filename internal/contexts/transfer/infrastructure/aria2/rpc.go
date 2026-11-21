package aria2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

type Client struct {
	metadataDir      string
	routes           downloadRoutes
	endpoint, secret string
	http             *http.Client
	sequence         atomic.Uint64
}
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("aria2 (%d): %s", e.Code, e.Message) }
func NewClient(endpoint, secret string) *Client {
	return &Client{endpoint: endpoint, secret: secret, http: &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 2}}}
}
func (c *Client) call(ctx context.Context, method string, params []any, result any) error {
	args := append([]any{"token:" + c.secret}, params...)
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.sequence.Add(1), "method": method, "params": args})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接下载引擎：%w", err)
	}
	defer resp.Body.Close()
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error"`
	}
	// aria2 also sends JSON-RPC errors (including an unknown GID) with HTTP 400.
	err = json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&envelope)
	if err == nil && envelope.Error != nil && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK) {
		return envelope.Error
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载引擎 HTTP 状态 %d", resp.StatusCode)
	}
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(envelope.Result, result)
	}
	return nil
}

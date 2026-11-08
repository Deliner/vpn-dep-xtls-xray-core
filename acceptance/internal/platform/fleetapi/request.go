package fleetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"vpn-script/internal/platform/credential"
)

func (c *Client) request(ctx context.Context, method, path, token string, value any, limit int64) ([]byte, error) {
	r, err := c.newRequest(ctx, method, path, token, value)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Fleet HTTPS request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusNoContent {
		return nil, &HTTPError{Status: response.StatusCode}
	}
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	kind, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		return nil, fmt.Errorf("Fleet response is not JSON")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("Fleet response read failed: %w", err)
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("Fleet response exceeds size limit")
	}
	return raw, nil
}

func (c *Client) newRequest(ctx context.Context, method, path, token string, value any) (*http.Request, error) {
	var raw []byte
	if value != nil {
		var err error
		raw, err = json.Marshal(value)
		if err != nil || len(raw) > 16<<20 {
			return nil, fmt.Errorf("invalid or oversized Fleet request")
		}
	}
	r, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if token != "" {
		if !credential.ValidToken(token) {
			return nil, fmt.Errorf("invalid Fleet credential")
		}
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if value != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Accept", "application/json")
	return r, nil
}

func decode(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid Fleet response structure")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing Fleet response data")
	}
	return nil
}

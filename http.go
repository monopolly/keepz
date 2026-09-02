package keepz

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *API) EcommerceRequest(ctx context.Context, method, path string, payload, out any) error {
	return c.doEncryptedJSON(ctx, method, c.ecommerceBaseURL, path, payload, nil, out)
}

func (c *API) DistributorRequest(ctx context.Context, method, path string, payload any, query url.Values, out any) error {
	return c.doDistributorJSON(ctx, method, path, payload, query, out)
}

func (c *API) doEncryptedJSON(ctx context.Context, method, baseURL, path string, payload any, query url.Values, out any) error {
	msg, err := c.encryptPayload(payload)
	if err != nil {
		return err
	}
	msg.Identifier = c.identifier

	if method == http.MethodGet || method == http.MethodDelete {
		if query == nil {
			query = url.Values{}
		}
		query.Set("identifier", msg.Identifier)
		query.Set("encryptedData", msg.EncryptedData)
		query.Set("encryptedKeys", msg.EncryptedKeys)
		query.Set("aes", "true")
		return c.doPlainJSON(ctx, method, baseURL, path, nil, query, encryptedResponseTarget{api: c, out: out})
	}
	return c.doPlainJSON(ctx, method, baseURL, path, msg, query, encryptedResponseTarget{api: c, out: out})
}

type encryptedResponseTarget struct {
	api *API
	out any
}

func (c *API) doPlainJSON(ctx context.Context, method, baseURL, path string, payload any, query url.Values, out any, headers ...string) error {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeKeepzError(resp.StatusCode, data)
	}
	if len(bytes.TrimSpace(data)) == 0 || out == nil {
		return nil
	}

	if target, ok := out.(encryptedResponseTarget); ok {
		var apiErr Error
		if json.Unmarshal(data, &apiErr) == nil && (apiErr.Message != "" || apiErr.StatusCode != 0) {
			apiErr.HTTPStatusCode = resp.StatusCode
			apiErr.Body = data
			return &apiErr
		}
		var msg encryptedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		plain, err := target.api.decryptMessage(msg)
		if err != nil {
			return err
		}
		if target.out == nil {
			return nil
		}
		return json.Unmarshal(plain, target.out)
	}

	return json.Unmarshal(data, out)
}

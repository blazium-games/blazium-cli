package guard

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const steamAPI = "https://api.steampowered.com"

type steamClient struct {
	http *http.Client
}

func newSteamClient() *steamClient {
	return &steamClient{http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *steamClient) call(service, method string, version int, body []byte, accessToken string) (*protoMap, int, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/v%d/", steamAPI, service, method, version))
	if err != nil {
		return nil, 0, err
	}
	q := u.Query()
	if accessToken != "" {
		q.Set("access_token", accessToken)
	}
	u.RawQuery = q.Encode()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("input_protobuf_encoded", base64.StdEncoding.EncodeToString(body)); err != nil {
		return nil, 0, err
	}
	if err := w.Close(); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), &buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	eresult := 0
	if v := resp.Header.Get("x-eresult"); v != "" {
		eresult, _ = strconv.Atoi(v)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, eresult, fmt.Errorf("steam unauthorized (x-eresult=%d)", eresult)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(resp.Header.Get("x-error_message"))
		if msg == "" {
			msg = string(raw)
		}
		return nil, eresult, fmt.Errorf("steam %s/%s HTTP %s: %s", service, method, resp.Status, msg)
	}
	if eresult != 0 && eresult != 1 {
		msg := strings.TrimSpace(resp.Header.Get("x-error_message"))
		if msg == "" {
			msg = fmt.Sprintf("x-eresult=%d", eresult)
		}
		return nil, eresult, fmt.Errorf("steam %s/%s: %s", service, method, msg)
	}
	m, err := decodeProto(raw)
	if err != nil {
		return nil, eresult, fmt.Errorf("decode %s/%s: %w", service, method, err)
	}
	return m, eresult, nil
}

// QueryTimeOffset returns Steam server_time minus local unix time.
func QueryTimeOffset() (int64, error) {
	c := newSteamClient()
	m, _, err := c.call("ITwoFactorService", "QueryTime", 1, nil, "")
	if err != nil {
		return 0, err
	}
	server := int64(m.u(1))
	if server == 0 {
		return 0, fmt.Errorf("QueryTime missing server_time")
	}
	return server - time.Now().Unix(), nil
}

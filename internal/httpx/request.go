package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type RequestSpec struct {
	Method  string
	BaseURL string
	URL     string
	Headers map[string]string
	Body    string
	JSON    any
	Form    map[string]string
	Timeout time.Duration
}

func ResolveURL(baseURL, u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u, nil
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("relative url %q requires base_url", u)
	}
	bu, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("bad base_url: %w", err)
	}
	uu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	if uu.Path != "" && !strings.HasPrefix(uu.Path, "/") {
		uu.Path = "/" + uu.Path
	}
	bu.Path = path.Clean(strings.TrimRight(bu.Path, "/") + uu.Path)
	bu.RawQuery = uu.RawQuery
	bu.Fragment = uu.Fragment
	return bu.String(), nil
}

func BuildRequest(ctx context.Context, spec RequestSpec) (*http.Request, []byte, error) {
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		return nil, nil, fmt.Errorf("empty method")
	}
	fullURL, err := ResolveURL(spec.BaseURL, spec.URL)
	if err != nil {
		return nil, nil, err
	}

	var body io.Reader
	var bodyBytes []byte
	if spec.JSON != nil {
		b, err := json.Marshal(spec.JSON)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal json body: %w", err)
		}
		bodyBytes = b
		body = bytes.NewReader(b)
	} else if len(spec.Form) > 0 {
		vals := make(url.Values, len(spec.Form))
		for k, v := range spec.Form {
			vals.Set(k, v)
		}
		enc := vals.Encode()
		bodyBytes = []byte(enc)
		body = strings.NewReader(enc)
	} else if spec.Body != "" {
		bodyBytes = []byte(spec.Body)
		body = strings.NewReader(spec.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range spec.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if spec.JSON != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if len(spec.Form) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return req, bodyBytes, nil
}

type Response struct {
	Status     int
	Headers    http.Header
	Body       []byte
	BodyIsJSON bool
}

func ReadResponse(resp *http.Response, maxBytes int64) (*Response, error) {
	lim := io.LimitReader(resp.Body, maxBytes)
	b, err := io.ReadAll(lim)
	closeErr := resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	ct := resp.Header.Get("Content-Type")
	mt, _, _ := mime.ParseMediaType(ct)
	isJSON := mt == "application/json" || strings.HasSuffix(mt, "+json")
	return &Response{
		Status:     resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       b,
		BodyIsJSON: isJSON,
	}, nil
}

func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}

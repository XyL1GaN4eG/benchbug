package httpx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type RequestSpec struct {
	Method, BaseURL, URL string
	Headers              map[string]string
	Body                 string
}

func ResolveURL(baseURL, u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u, nil
	}
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("relative url requires base_url")
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(u, "/"), nil
}

func BuildRequest(ctx context.Context, spec RequestSpec) (*http.Request, error) {
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}
	u, err := ResolveURL(spec.BaseURL, spec.URL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

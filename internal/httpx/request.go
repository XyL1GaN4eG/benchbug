package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
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
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	return req, nil
}

package scenario

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type TemplateCtx struct {
	VU   int
	Iter int64
	Rand *rand.Rand
}

func Expand(s string, vars map[string]string, ctx TemplateCtx) (string, error) {
	return expandDepth(s, vars, ctx, 0)
}

func expandDepth(s string, vars map[string]string, ctx TemplateCtx, depth int) (string, error) {
	if depth > 8 {
		return "", fmt.Errorf("template expansion depth exceeded")
	}
	if s == "" {
		return s, nil
	}
	var out strings.Builder
	out.Grow(len(s))
	for {
		i := strings.Index(s, "${")
		if i < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:i])
		s = s[i+2:]
		j := strings.IndexByte(s, '}')
		if j < 0 {
			return "", fmt.Errorf("unterminated template placeholder")
		}
		key := s[:j]
		s = s[j+1:]

		val, ok, err := evalPlaceholder(strings.TrimSpace(key), vars, ctx, depth)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("unknown template variable %q", key)
		}
		out.WriteString(val)
	}
	return out.String(), nil
}

func evalPlaceholder(key string, vars map[string]string, ctx TemplateCtx, depth int) (string, bool, error) {
	switch key {
	case "__vu":
		return strconv.Itoa(ctx.VU), true, nil
	case "__iter":
		return strconv.FormatInt(ctx.Iter, 10), true, nil
	}
	if strings.HasPrefix(key, "__rand_int(") && strings.HasSuffix(key, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(key, "__rand_int("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) != 2 {
			return "", false, fmt.Errorf("bad __rand_int(min,max) args: %q", inner)
		}
		a, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return "", false, fmt.Errorf("bad __rand_int min: %w", err)
		}
		b, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return "", false, fmt.Errorf("bad __rand_int max: %w", err)
		}
		if a > b {
			a, b = b, a
		}
		r := ctx.Rand
		if r == nil {
			r = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		n := a + r.Intn((b-a)+1)
		return strconv.Itoa(n), true, nil
	}

	v, ok := vars[key]
	if !ok {
		return "", false, nil
	}
	if strings.Contains(v, "${") {
		ev, err := expandDepth(v, vars, ctx, depth+1)
		return ev, true, err
	}
	return v, true, nil
}

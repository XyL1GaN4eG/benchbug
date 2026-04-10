package scenario

import (
	"fmt"
	"strconv"
	"strings"
)

type TemplateCtx struct {
	VU   int
	Iter int64
}

func Expand(s string, vars map[string]string, ctx TemplateCtx) (string, error) {
	var out strings.Builder
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
		key := strings.TrimSpace(s[:j])
		s = s[j+1:]
		switch key {
		case "__vu":
			out.WriteString(strconv.Itoa(ctx.VU))
		case "__iter":
			out.WriteString(strconv.FormatInt(ctx.Iter, 10))
		default:
			v, ok := vars[key]
			if !ok {
				return "", fmt.Errorf("unknown template variable %q", key)
			}
			out.WriteString(v)
		}
	}
	return out.String(), nil
}

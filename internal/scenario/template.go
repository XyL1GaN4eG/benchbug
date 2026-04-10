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
	out := s
	out = strings.ReplaceAll(out, "${__vu}", strconv.Itoa(ctx.VU))
	out = strings.ReplaceAll(out, "${__iter}", strconv.FormatInt(ctx.Iter, 10))
	for k, v := range vars {
		out = strings.ReplaceAll(out, "${"+k+"}", v)
	}
	if strings.Contains(out, "${") {
		return "", fmt.Errorf("unknown template variable")
	}
	return out, nil
}

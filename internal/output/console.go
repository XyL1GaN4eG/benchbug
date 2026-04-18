package output

import (
	"fmt"
	"io"

	"benchbug/internal/metrics"
)

type Console struct{ w io.Writer }

func NewConsole(w io.Writer) *Console { return &Console{w: w} }
func (c *Console) OnSummary(sum metrics.Summary) {
	fmt.Fprintf(c.w, "SUMMARY requests=%d fails=%d p95=%s\n", sum.Requests, sum.Fails, sum.P95)
}

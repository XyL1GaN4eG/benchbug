package scenario

import (
	"fmt"
	"time"
)

type Duration struct {
	time.Duration
}

func (d Duration) String() string {
	return d.Duration.String()
}

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var v any
	if err := unmarshal(&v); err != nil {
		return err
	}
	return d.unmarshalAny(v)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		d.Duration = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := unmarshalJSONInto(b, &s); err != nil {
			return err
		}
		return d.unmarshalAny(s)
	}
	var n int64
	if err := unmarshalJSONInto(b, &n); err != nil {
		return err
	}
	d.Duration = time.Duration(n) * time.Millisecond
	return nil
}

func (d *Duration) unmarshalAny(v any) error {
	switch x := v.(type) {
	case nil:
		d.Duration = 0
		return nil
	case int:
		d.Duration = time.Duration(x) * time.Millisecond
		return nil
	case int64:
		d.Duration = time.Duration(x) * time.Millisecond
		return nil
	case float64:
		d.Duration = time.Duration(int64(x)) * time.Millisecond
		return nil
	case string:
		if x == "" {
			d.Duration = 0
			return nil
		}
		dd, err := time.ParseDuration(x)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", x, err)
		}
		d.Duration = dd
		return nil
	default:
		return fmt.Errorf("unsupported duration type %T", v)
	}
}

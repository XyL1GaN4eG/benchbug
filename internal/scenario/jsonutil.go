package scenario

import "encoding/json"

func unmarshalJSONInto[T any](b []byte, out *T) error {
	return json.Unmarshal(b, out)
}

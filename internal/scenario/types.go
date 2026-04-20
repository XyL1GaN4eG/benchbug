package scenario

type Scenario struct {
	Name      string            `yaml:"name" json:"name"`
	BaseURL   string            `yaml:"base_url" json:"base_url"`
	VUs       int               `yaml:"vus" json:"vus"`
	Duration  Duration          `yaml:"duration" json:"duration"`
	Stages    []Stage           `yaml:"stages" json:"stages"`
	Arrival   *ArrivalRate      `yaml:"arrival_rate" json:"arrival_rate"`
	Defaults  Defaults          `yaml:"defaults" json:"defaults"`
	Vars      map[string]string `yaml:"vars" json:"vars"`
	Steps     []Step            `yaml:"steps" json:"steps"`
	Threshold []Threshold       `yaml:"thresholds" json:"thresholds"`
}

type Stage struct {
	Duration Duration `yaml:"duration" json:"duration"`
	VUs      int      `yaml:"vus" json:"vus"`
}

type ArrivalRate struct {
	Rate     float64  `yaml:"rate" json:"rate"`
	Per      Duration `yaml:"per" json:"per"`
	Duration Duration `yaml:"duration" json:"duration"`
	MaxVUs   int      `yaml:"max_vus" json:"max_vus"`
}

type Defaults struct {
	Headers map[string]string `yaml:"headers" json:"headers"`
	Timeout Duration          `yaml:"timeout" json:"timeout"`
	Tags    map[string]string `yaml:"tags" json:"tags"`
}

type Step struct {
	Name      string            `yaml:"name" json:"name"`
	Group     string            `yaml:"group" json:"group"`
	Request   Request           `yaml:"request" json:"request"`
	ThinkTime Duration          `yaml:"think_time" json:"think_time"`
	Extract   Extract           `yaml:"extract" json:"extract"`
	Checks    []Check           `yaml:"checks" json:"checks"`
	Tags      map[string]string `yaml:"tags" json:"tags"`
}

type Request struct {
	Method  string            `yaml:"method" json:"method"`
	URL     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Body    string            `yaml:"body" json:"body"`
	JSON    any               `yaml:"json" json:"json"`
	Form    map[string]string `yaml:"form" json:"form"`
	Timeout Duration          `yaml:"timeout" json:"timeout"`
}

type Extract struct {
	JSONPath map[string]string `yaml:"jsonpath" json:"jsonpath"`
	Header   map[string]string `yaml:"header" json:"header"`
}

type Check struct {
	StatusIn       []int          `yaml:"status_in" json:"status_in"`
	JSONPathExists string         `yaml:"jsonpath_exists" json:"jsonpath_exists"`
	JSONPathEq     *JSONPathEq    `yaml:"jsonpath_eq" json:"jsonpath_eq"`
	HeaderEq       map[string]any `yaml:"header_eq" json:"header_eq"`
}

type JSONPathEq struct {
	Path  string `yaml:"path" json:"path"`
	Value any    `yaml:"value" json:"value"`
}

type Threshold struct {
	Metric string `yaml:"metric" json:"metric"`
	Op     string `yaml:"op" json:"op"`
	Value  any    `yaml:"value" json:"value"`
}

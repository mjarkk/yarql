package yarql

import (
	"encoding/json"
	"time"
)

type tracer struct {
	Version     uint8                  `json:"version"`
	GoStartTime time.Time              `json:"-"`
	StartTime   string                 `json:"startTime"`
	EndTime     string                 `json:"endTime"`
	Duration    int64                  `json:"duration"`
	Parsing     tracerStartAndDuration `json:"parsing"`
	Validation  tracerStartAndDuration `json:"validation"`
	Execution   tracerExecution        `json:"execution"`
}

type tracerStartAndDuration struct {
	StartOffset int64 `json:"startOffset"`
	Duration    int64 `json:"duration"`
}

type tracerExecution struct {
	Resolvers []tracerResolver `json:"resolvers"`
}

type tracerResolver struct {
	Path        json.RawMessage `json:"path"`
	ParentType  string          `json:"parentType"`
	FieldName   string          `json:"fieldName"`
	ReturnType  string          `json:"returnType"`
	StartOffset int64           `json:"startOffset"`
	Duration    int64           `json:"duration"`
}

func newTracer() *tracer {
	return &tracer{
		Version:     1,
		GoStartTime: time.Now(),
		Execution: tracerExecution{
			Resolvers: []tracerResolver{},
		},
	}
}

func (t *tracer) reset() {
	*t = tracer{
		Version:     1,
		GoStartTime: time.Now(),
		Execution: tracerExecution{
			Resolvers: t.Execution.Resolvers[:0],
		},
	}
}

func (t *tracer) finish() {
	t.StartTime = t.GoStartTime.Format(time.RFC3339Nano)
	now := time.Now()
	t.EndTime = now.Format(time.RFC3339Nano)
	t.Duration = now.Sub(t.GoStartTime).Nanoseconds()
}

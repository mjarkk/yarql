package graphql

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

func (t *tracer) finish() {
	t.StartTime = t.GoStartTime.Format(time.RFC3339Nano)
	now := time.Now()
	t.EndTime = now.Format(time.RFC3339Nano)
	t.Duration = now.Sub(t.GoStartTime).Nanoseconds()
}

func (ctx *Ctx) finishTrace(report func(offset, duration int64)) {
	if ctx.tracingEnabled {
		f := ctx.prefRecordingStartTime
		offset := f.Sub(ctx.tracing.GoStartTime).Nanoseconds()
		duration := time.Since(f).Nanoseconds()
		report(offset, duration)
	}
}

func (ctx *Ctx) startTrace() {
	if ctx.tracingEnabled {
		ctx.prefRecordingStartTime = time.Now()
	}
}

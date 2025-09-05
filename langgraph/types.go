package langgraph

import (
	"context"
	"time"

	"github.com/tmc/langgraphgo/graph"
)

// Runnable interface for objects that can be invoked
type Runnable interface {
	Invoke(ctx context.Context, initialState interface{}) (interface{}, error)
}

// TraceContext holds contextual information for tracing
type TraceContext struct {
	TraceID      string
	ParentSpanID string
	UserID       string
	SessionID    string
	Metadata     map[string]interface{}
	Tags         []string
}

// NodeInfo contains information about a graph node
type NodeInfo struct {
	Name       string
	Type       NodeType
	Model      string
	Parameters map[string]interface{}
}

// NodeType represents the type of a graph node
type NodeType int

const (
	NodeTypeUnknown NodeType = iota
	NodeTypeAI
	NodeTypeTool
	NodeTypeCondition
	NodeTypeTransform
	NodeTypeStart
	NodeTypeEnd
)

// String returns the string representation of NodeType
func (t NodeType) String() string {
	switch t {
	case NodeTypeAI:
		return "ai"
	case NodeTypeTool:
		return "tool"
	case NodeTypeCondition:
		return "condition"
	case NodeTypeTransform:
		return "transform"
	case NodeTypeStart:
		return "start"
	case NodeTypeEnd:
		return "end"
	default:
		return "unknown"
	}
}

// TraceHookBuilder provides a fluent interface for building hooks
type TraceHookBuilder struct {
	hook *Hook
}

// NewBuilder creates a new hook builder
func NewBuilder() *TraceHookBuilder {
	return &TraceHookBuilder{
		hook: NewHook(),
	}
}

// WithAutoFlush configures auto-flushing
func (b *TraceHookBuilder) WithAutoFlush(enabled bool) *TraceHookBuilder {
	b.hook.config.AutoFlush = enabled
	return b
}

// WithMetadata adds metadata
func (b *TraceHookBuilder) WithMetadata(metadata map[string]interface{}) *TraceHookBuilder {
	b.hook.config.DefaultMetadata = metadata
	return b
}

// WithTraceName sets the trace name
func (b *TraceHookBuilder) WithTraceName(name string) *TraceHookBuilder {
	b.hook.config.TraceName = name
	return b
}

// WithSessionID sets the session ID
func (b *TraceHookBuilder) WithSessionID(id string) *TraceHookBuilder {
	b.hook.config.SessionID = id
	return b
}

// WithUserID sets the user ID
func (b *TraceHookBuilder) WithUserID(id string) *TraceHookBuilder {
	b.hook.config.UserID = id
	return b
}

// WithTags adds tags
func (b *TraceHookBuilder) WithTags(tags ...string) *TraceHookBuilder {
	b.hook.config.Tags = tags
	return b
}

// Build returns the configured hook
func (b *TraceHookBuilder) Build() *Hook {
	return b.hook
}

// TracedRunnable wraps a runnable with tracing
type TracedRunnable struct {
	runnable Runnable
	tracer   *graph.Tracer
	hooks    []graph.TraceHook
}

// NewTracedRunnable creates a new traced runnable
func NewTracedRunnable(runnable Runnable, hooks ...graph.TraceHook) *TracedRunnable {
	tracer := graph.NewTracer()
	for _, hook := range hooks {
		tracer.AddHook(hook)
	}

	return &TracedRunnable{
		runnable: runnable,
		tracer:   tracer,
		hooks:    hooks,
	}
}

// Invoke executes the runnable with tracing
func (t *TracedRunnable) Invoke(ctx context.Context, input interface{}) (interface{}, error) {
	// Set initial input for hooks that support it
	for _, hook := range t.hooks {
		if h, isHook := hook.(*Hook); isHook {
			h.SetInitialInput(input)
		}
	}

	// For graph.Runnable, use the traced version from langgraphgo
	if graphRunnable, isGraphRunnable := t.runnable.(*graph.Runnable); isGraphRunnable {
		traced := graph.NewTracedRunnable(graphRunnable, t.tracer)
		return traced.Invoke(ctx, input)
	}

	// For graph.StateRunnable, execute directly (no traced version available)
	if stateRunnable, isStateRunnable := t.runnable.(*graph.StateRunnable); isStateRunnable {
		return stateRunnable.Invoke(ctx, input)
	}

	// For other runnables, execute directly without langgraphgo tracing
	// (the hooks will still receive events via other mechanisms if implemented)
	return t.runnable.Invoke(ctx, input)
}

// Stream executes the runnable with streaming and tracing
func (t *TracedRunnable) Stream(ctx context.Context, input interface{}) (<-chan interface{}, <-chan error) {
	// Set initial input for hooks that support it
	for _, hook := range t.hooks {
		if h, isHookType := hook.(*Hook); isHookType {
			h.SetInitialInput(input)
		}
	}

	// Execute with tracing - use the same logic as Invoke
	result, err := t.Invoke(ctx, input)
	ch := make(chan interface{}, 1)
	errCh := make(chan error, 1)

	go func() {
		if err != nil {
			errCh <- err
		} else {
			ch <- result
		}
		close(ch)
		close(errCh)
	}()

	return ch, errCh
}

// EventFilter allows filtering of trace events
type EventFilter struct {
	// IncludeEvents specifies which events to include
	IncludeEvents []graph.TraceEvent
	// ExcludeEvents specifies which events to exclude
	ExcludeEvents []graph.TraceEvent
	// MinDuration filters out spans shorter than this duration
	MinDuration time.Duration
}

// FilteredHook wraps a hook with event filtering
type FilteredHook struct {
	hook   graph.TraceHook
	filter EventFilter
}

// NewFilteredHook creates a new filtered hook
func NewFilteredHook(hook graph.TraceHook, filter EventFilter) *FilteredHook {
	return &FilteredHook{
		hook:   hook,
		filter: filter,
	}
}

// OnEvent filters events before passing to the wrapped hook
func (f *FilteredHook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	// Check if event should be excluded
	for _, event := range f.filter.ExcludeEvents {
		if span.Event == event {
			return
		}
	}

	// Check if event should be included (if include list is specified)
	if len(f.filter.IncludeEvents) > 0 {
		included := false
		for _, event := range f.filter.IncludeEvents {
			if span.Event == event {
				included = true
				break
			}
		}
		if !included {
			return
		}
	}

	// Check duration filter for end events
	if span.Event == graph.TraceEventNodeEnd || span.Event == graph.TraceEventGraphEnd {
		if f.filter.MinDuration > 0 && span.Duration < f.filter.MinDuration {
			return
		}
	}

	// Pass through to wrapped hook
	f.hook.OnEvent(ctx, span)
}

// MultiHook combines multiple trace hooks
type MultiHook struct {
	hooks []graph.TraceHook
}

// NewMultiHook creates a new multi-hook
func NewMultiHook(hooks ...graph.TraceHook) *MultiHook {
	return &MultiHook{hooks: hooks}
}

// OnEvent sends events to all hooks
func (m *MultiHook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	for _, hook := range m.hooks {
		hook.OnEvent(ctx, span)
	}
}

// AddHook adds a hook to the multi-hook
func (m *MultiHook) AddHook(hook graph.TraceHook) {
	m.hooks = append(m.hooks, hook)
}

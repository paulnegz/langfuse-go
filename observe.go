package langfuse

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/paulnegz/langfuse-go/model"
)

// ObservationType represents the type of observation
type ObservationType string

const (
	ObservationTypeGeneration ObservationType = "generation"
	ObservationTypeSpan       ObservationType = "span"
	ObservationTypeAgent      ObservationType = "agent"
	ObservationTypeTool       ObservationType = "tool"
	ObservationTypeChain      ObservationType = "chain"
	ObservationTypeRetriever  ObservationType = "retriever"
	ObservationTypeEmbedding  ObservationType = "embedding"
	ObservationTypeEvaluator  ObservationType = "evaluator"
	ObservationTypeGuardrail  ObservationType = "guardrail"
)

// Observer provides function observation capabilities similar to Python's @observe decorator
type Observer struct {
	client      *Langfuse
	traceID     string
	parentID    *string
	sessionID   string
	userID      string
	name        string
	obsType     ObservationType
	metadata    map[string]interface{}
	captureIO   bool
	sampleRate  float64
}

// ObserveOption configures the observer
type ObserveOption func(*Observer)

// WithObservationType sets the observation type
func WithObservationType(t ObservationType) ObserveOption {
	return func(o *Observer) {
		o.obsType = t
	}
}

// WithObserveName sets a custom name for the observation
func WithObserveName(name string) ObserveOption {
	return func(o *Observer) {
		o.name = name
	}
}

// WithObserveMetadata adds metadata to observations
func WithObserveMetadata(metadata map[string]interface{}) ObserveOption {
	return func(o *Observer) {
		o.metadata = metadata
	}
}

// WithObserveSession sets the session ID
func WithObserveSession(sessionID string) ObserveOption {
	return func(o *Observer) {
		o.sessionID = sessionID
	}
}

// WithObserveUser sets the user ID
func WithObserveUser(userID string) ObserveOption {
	return func(o *Observer) {
		o.userID = userID
	}
}

// WithCaptureIO enables/disables input/output capture
func WithCaptureIO(capture bool) ObserveOption {
	return func(o *Observer) {
		o.captureIO = capture
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0)
func WithSampleRate(rate float64) ObserveOption {
	return func(o *Observer) {
		o.sampleRate = rate
	}
}

// NewObserver creates a new observer instance
func NewObserver(client *Langfuse, opts ...ObserveOption) *Observer {
	o := &Observer{
		client:     client,
		obsType:    ObservationTypeSpan,
		metadata:   make(map[string]interface{}),
		captureIO:  true,
		sampleRate: 1.0,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Observe wraps a function with observation capabilities
// This is the Go equivalent of Python's @observe decorator
func (o *Observer) Observe(fn interface{}) interface{} {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("Observe: argument must be a function")
	}

	// Get function name if not specified
	if o.name == "" {
		o.name = runtime.FuncForPC(fnValue.Pointer()).Name()
	}

	// Create wrapped function
	wrappedFn := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		// Check sampling
		if !o.shouldSample() {
			return fnValue.Call(args)
		}

		// Start observation
		ctx := context.Background()
		startTime := time.Now()
		
		// Create trace if needed
		if o.traceID == "" {
			trace := &model.Trace{
				ID:        uuid.New().String(),
				Name:      o.name,
				Timestamp: &startTime,
				SessionID: o.sessionID,
				UserID:    o.userID,
				Metadata:  o.metadata,
			}
			
			createdTrace, err := o.client.Trace(trace)
			if err == nil && createdTrace != nil {
				o.traceID = createdTrace.ID
			}
		}

		// Capture input if enabled
		var input interface{}
		if o.captureIO && len(args) > 0 {
			input = o.captureArgs(args)
		}

		// Create observation based on type
		var observationID string
		switch o.obsType {
		case ObservationTypeGeneration:
			gen := &model.Generation{
				ID:        uuid.New().String(),
				TraceID:   o.traceID,
				Name:      o.name,
				StartTime: &startTime,
				Input:     input,
				Metadata:  o.metadata,
			}
			
			createdGen, err := o.client.Generation(gen, o.parentID)
			if err == nil && createdGen != nil {
				observationID = createdGen.ID
			}
			
		default: // Span or other types
			span := &model.Span{
				ID:        uuid.New().String(),
				TraceID:   o.traceID,
				Name:      o.name,
				StartTime: &startTime,
				Input:     input,
				Metadata:  o.metadata,
			}
			
			createdSpan, err := o.client.Span(span, o.parentID)
			if err == nil && createdSpan != nil {
				observationID = createdSpan.ID
			}
		}

		// Execute the function
		results := fnValue.Call(args)
		
		// Capture output if enabled
		var output interface{}
		var fnErr error
		if o.captureIO && len(results) > 0 {
			output, fnErr = o.captureResults(results)
		}

		// End observation
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		
		// Update observation with results
		switch o.obsType {
		case ObservationTypeGeneration:
			o.client.GenerationEnd(&model.Generation{
				ID:      observationID,
				TraceID: o.traceID,
				EndTime: &endTime,
				Output:  output,
				Metadata: map[string]interface{}{
					"duration_ms": duration.Milliseconds(),
					"error":       fnErr != nil,
				},
			})
			
		default:
			o.client.SpanEnd(&model.Span{
				ID:      observationID,
				TraceID: o.traceID,
				EndTime: &endTime,
				Output:  output,
				Metadata: map[string]interface{}{
					"duration_ms": duration.Milliseconds(),
					"error":       fnErr != nil,
				},
			})
		}

		// Create child observer for nested calls
		if observationID != "" {
			// Store parent ID in context for nested observations
			ctx = context.WithValue(ctx, "langfuse_parent_id", observationID)
		}

		return results
	})

	return wrappedFn.Interface()
}

// ObserveFunc is a convenience function to wrap and execute a function with observation
func ObserveFunc(client *Langfuse, fn func() error, opts ...ObserveOption) error {
	observer := NewObserver(client, opts...)
	wrappedFn := observer.Observe(fn).(func() error)
	return wrappedFn()
}

// ObserveWithResult wraps a function that returns a value and error
func ObserveWithResult[T any](client *Langfuse, fn func() (T, error), opts ...ObserveOption) (T, error) {
	observer := NewObserver(client, opts...)
	wrappedFn := observer.Observe(fn).(func() (T, error))
	return wrappedFn()
}

// shouldSample determines if this observation should be sampled
func (o *Observer) shouldSample() bool {
	if o.sampleRate >= 1.0 {
		return true
	}
	if o.sampleRate <= 0.0 {
		return false
	}
	// Simple random sampling
	return float64(time.Now().UnixNano()%100)/100.0 < o.sampleRate
}

// captureArgs converts function arguments to a capturable format
func (o *Observer) captureArgs(args []reflect.Value) interface{} {
	if len(args) == 1 {
		return o.reflectValueToInterface(args[0])
	}
	
	captured := make([]interface{}, len(args))
	for i, arg := range args {
		captured[i] = o.reflectValueToInterface(arg)
	}
	return captured
}

// captureResults converts function results to a capturable format
func (o *Observer) captureResults(results []reflect.Value) (interface{}, error) {
	if len(results) == 0 {
		return nil, nil
	}
	
	// Check if last result is an error
	var err error
	if len(results) > 0 {
		lastResult := results[len(results)-1]
		if lastResult.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !lastResult.IsNil() {
				err = lastResult.Interface().(error)
			}
		}
	}
	
	// If only error result
	if len(results) == 1 && err != nil {
		return nil, err
	}
	
	// If single non-error result
	if len(results) == 1 {
		return o.reflectValueToInterface(results[0]), nil
	}
	
	// Multiple results
	captured := make([]interface{}, 0, len(results))
	for i, result := range results {
		// Skip error result
		if i == len(results)-1 && err != nil {
			continue
		}
		captured = append(captured, o.reflectValueToInterface(result))
	}
	
	if len(captured) == 1 {
		return captured[0], err
	}
	return captured, err
}

// reflectValueToInterface safely converts a reflect.Value to interface{}
func (o *Observer) reflectValueToInterface(v reflect.Value) interface{} {
	if !v.IsValid() {
		return nil
	}
	
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}
	
	// Handle special types
	switch v.Kind() {
	case reflect.Func:
		return fmt.Sprintf("function<%s>", v.Type().String())
	case reflect.Chan:
		return fmt.Sprintf("channel<%s>", v.Type().String())
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return v.Interface()
	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return fmt.Sprintf("<%s>", v.Type().String())
	}
}

// ObserveContext creates an observation context for manual span management
type ObserveContext struct {
	observer      *Observer
	observationID string
	startTime     time.Time
	obsType       ObservationType
}

// Start begins a new observation
func (o *Observer) Start(name string) *ObserveContext {
	startTime := time.Now()
	
	// Create trace if needed
	if o.traceID == "" {
		trace := &model.Trace{
			ID:        uuid.New().String(),
			Name:      name,
			Timestamp: &startTime,
			SessionID: o.sessionID,
			UserID:    o.userID,
			Metadata:  o.metadata,
		}
		
		createdTrace, _ := o.client.Trace(trace)
		if createdTrace != nil {
			o.traceID = createdTrace.ID
		}
	}
	
	// Create observation
	observationID := uuid.New().String()
	switch o.obsType {
	case ObservationTypeGeneration:
		gen := &model.Generation{
			ID:        observationID,
			TraceID:   o.traceID,
			Name:      name,
			StartTime: &startTime,
			Metadata:  o.metadata,
		}
		o.client.Generation(gen, o.parentID)
		
	default:
		span := &model.Span{
			ID:        observationID,
			TraceID:   o.traceID,
			Name:      name,
			StartTime: &startTime,
			Metadata:  o.metadata,
		}
		o.client.Span(span, o.parentID)
	}
	
	return &ObserveContext{
		observer:      o,
		observationID: observationID,
		startTime:     startTime,
		obsType:       o.obsType,
	}
}

// End completes an observation
func (oc *ObserveContext) End(output interface{}, err error) {
	endTime := time.Now()
	duration := endTime.Sub(oc.startTime)
	
	metadata := map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	
	switch oc.obsType {
	case ObservationTypeGeneration:
		oc.observer.client.GenerationEnd(&model.Generation{
			ID:       oc.observationID,
			TraceID:  oc.observer.traceID,
			EndTime:  &endTime,
			Output:   output,
			Metadata: metadata,
		})
		
	default:
		oc.observer.client.SpanEnd(&model.Span{
			ID:       oc.observationID,
			TraceID:  oc.observer.traceID,
			EndTime:  &endTime,
			Output:   output,
			Metadata: metadata,
		})
	}
}

// WithObserver adds an observer to the context
func WithObserver(ctx context.Context, observer *Observer) context.Context {
	return context.WithValue(ctx, "langfuse_observer", observer)
}

// ObserverFromContext retrieves an observer from context
func ObserverFromContext(ctx context.Context) *Observer {
	if observer, ok := ctx.Value("langfuse_observer").(*Observer); ok {
		return observer
	}
	return nil
}
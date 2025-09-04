package langgraph

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tmc/langgraphgo/graph"
)

// MockLangfuseClient for testing
type MockLangfuseClient struct {
	traces      []interface{}
	spans       []interface{}
	generations []interface{}
	flushed     bool
}

func (m *MockLangfuseClient) RecordTrace(trace interface{}) {
	m.traces = append(m.traces, trace)
}

func (m *MockLangfuseClient) RecordSpan(span interface{}) {
	m.spans = append(m.spans, span)
}

func (m *MockLangfuseClient) RecordGeneration(gen interface{}) {
	m.generations = append(m.generations, gen)
}

func (m *MockLangfuseClient) Flush() {
	m.flushed = true
}

// Test hook creation
func TestNewHook(t *testing.T) {
	tests := []struct {
		name     string
		options  []Option
		expected Config
	}{
		{
			name:    "Default configuration",
			options: []Option{},
			expected: Config{
				AutoFlush:       true,
				TraceName:       "langgraph_workflow",
				Tags:            []string{"golang", "langgraph"},
				DefaultMetadata: make(map[string]interface{}),
			},
		},
		{
			name: "Custom configuration",
			options: []Option{
				WithTraceName("custom_workflow"),
				WithSessionID("session-123"),
				WithUserID("user-456"),
				WithAutoFlush(false),
				WithTags([]string{"test", "custom"}),
			},
			expected: Config{
				AutoFlush:       false,
				TraceName:       "custom_workflow",
				SessionID:       "session-123",
				UserID:          "user-456",
				Tags:            []string{"test", "custom"},
				DefaultMetadata: make(map[string]interface{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := NewHook(tt.options...)
			
			if hook.config.TraceName != tt.expected.TraceName {
				t.Errorf("TraceName: got %v, want %v", hook.config.TraceName, tt.expected.TraceName)
			}
			if hook.config.SessionID != tt.expected.SessionID {
				t.Errorf("SessionID: got %v, want %v", hook.config.SessionID, tt.expected.SessionID)
			}
			if hook.config.UserID != tt.expected.UserID {
				t.Errorf("UserID: got %v, want %v", hook.config.UserID, tt.expected.UserID)
			}
			if hook.config.AutoFlush != tt.expected.AutoFlush {
				t.Errorf("AutoFlush: got %v, want %v", hook.config.AutoFlush, tt.expected.AutoFlush)
			}
		})
	}
}

// Test builder pattern
func TestTraceHookBuilder(t *testing.T) {
	hook := NewBuilder().
		WithTraceName("builder_test").
		WithSessionID("session-builder").
		WithUserID("user-builder").
		WithMetadata(map[string]interface{}{
			"test": true,
		}).
		WithTags("builder", "test").
		WithAutoFlush(false).
		Build()

	if hook.config.TraceName != "builder_test" {
		t.Errorf("TraceName: got %v, want builder_test", hook.config.TraceName)
	}
	if hook.config.SessionID != "session-builder" {
		t.Errorf("SessionID: got %v, want session-builder", hook.config.SessionID)
	}
	if hook.config.UserID != "user-builder" {
		t.Errorf("UserID: got %v, want user-builder", hook.config.UserID)
	}
	if hook.config.AutoFlush != false {
		t.Errorf("AutoFlush: got %v, want false", hook.config.AutoFlush)
	}
}

// Test SetInitialInput
func TestSetInitialInput(t *testing.T) {
	hook := NewHook()
	
	input := map[string]interface{}{
		"key": "value",
		"number": 42,
	}
	
	hook.SetInitialInput(input)
	
	if hook.initialInput == nil {
		t.Error("Initial input was not set")
	}
	
	if inputMap, ok := hook.initialInput.(map[string]interface{}); ok {
		if inputMap["key"] != "value" {
			t.Errorf("Input key: got %v, want value", inputMap["key"])
		}
		if inputMap["number"] != 42 {
			t.Errorf("Input number: got %v, want 42", inputMap["number"])
		}
	} else {
		t.Error("Initial input is not the expected type")
	}
}

// Test AI operation detection
func TestIsAIOperation(t *testing.T) {
	hook := NewHook()
	
	tests := []struct {
		nodeName string
		expected bool
	}{
		{"generate_response", true},
		{"ai_completion", true},
		{"llm_call", true},
		{"chat_response", true},
		{"gpt_generation", true},
		{"claude_analysis", true},
		{"gemini_response", true},
		{"openai_completion", true},
		{"process_data", false},
		{"validate_input", false},
		{"transform_output", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.nodeName, func(t *testing.T) {
			result := hook.isAIOperation(tt.nodeName)
			if result != tt.expected {
				t.Errorf("isAIOperation(%s): got %v, want %v", tt.nodeName, result, tt.expected)
			}
		})
	}
}

// Test model extraction
func TestExtractModel(t *testing.T) {
	hook := NewHook()
	
	tests := []struct {
		name     string
		span     *graph.TraceSpan
		expected string
	}{
		{
			name: "Model in metadata",
			span: &graph.TraceSpan{
				NodeName: "generate",
				Metadata: map[string]interface{}{
					"model": "gpt-4",
				},
			},
			expected: "gpt-4",
		},
		{
			name: "GPT pattern",
			span: &graph.TraceSpan{
				NodeName: "gpt_generation",
			},
			expected: "gpt-3.5-turbo",
		},
		{
			name: "Claude pattern",
			span: &graph.TraceSpan{
				NodeName: "claude_response",
			},
			expected: "claude-3-sonnet",
		},
		{
			name: "Gemini pattern",
			span: &graph.TraceSpan{
				NodeName: "gemini_analysis",
			},
			expected: "gemini-pro",
		},
		{
			name: "Unknown",
			span: &graph.TraceSpan{
				NodeName: "process",
			},
			expected: "unknown",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hook.extractModel(tt.span)
			if result != tt.expected {
				t.Errorf("extractModel: got %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test event filter
func TestFilteredHook(t *testing.T) {
	baseHook := &MockTraceHook{
		events: []graph.TraceEvent{},
	}
	
	filter := EventFilter{
		IncludeEvents: []graph.TraceEvent{
			graph.TraceEventNodeStart,
			graph.TraceEventNodeEnd,
		},
		MinDuration: 100 * time.Millisecond,
	}
	
	filteredHook := NewFilteredHook(baseHook, filter)
	ctx := context.Background()
	
	// Event that should be filtered out (wrong type)
	span1 := &graph.TraceSpan{
		Event: graph.TraceEventEdgeTraversal,
	}
	filteredHook.OnEvent(ctx, span1)
	
	if len(baseHook.events) != 0 {
		t.Error("Edge traversal event should be filtered out")
	}
	
	// Event that should pass (correct type)
	span2 := &graph.TraceSpan{
		Event:    graph.TraceEventNodeStart,
		NodeName: "test_node",
	}
	filteredHook.OnEvent(ctx, span2)
	
	if len(baseHook.events) != 1 {
		t.Error("Node start event should pass through")
	}
	
	// Event that should be filtered out (too short duration)
	span3 := &graph.TraceSpan{
		Event:    graph.TraceEventNodeEnd,
		Duration: 50 * time.Millisecond,
	}
	filteredHook.OnEvent(ctx, span3)
	
	if len(baseHook.events) != 1 {
		t.Error("Short duration event should be filtered out")
	}
	
	// Event that should pass (long duration)
	span4 := &graph.TraceSpan{
		Event:    graph.TraceEventNodeEnd,
		Duration: 200 * time.Millisecond,
	}
	filteredHook.OnEvent(ctx, span4)
	
	if len(baseHook.events) != 2 {
		t.Error("Long duration event should pass through")
	}
}

// Test multi-hook
func TestMultiHook(t *testing.T) {
	hook1 := &MockTraceHook{events: []graph.TraceEvent{}}
	hook2 := &MockTraceHook{events: []graph.TraceEvent{}}
	hook3 := &MockTraceHook{events: []graph.TraceEvent{}}
	
	multiHook := NewMultiHook(hook1, hook2, hook3)
	ctx := context.Background()
	
	span := &graph.TraceSpan{
		Event:    graph.TraceEventNodeStart,
		NodeName: "test",
	}
	
	multiHook.OnEvent(ctx, span)
	
	if len(hook1.events) != 1 {
		t.Error("Hook1 should receive event")
	}
	if len(hook2.events) != 1 {
		t.Error("Hook2 should receive event")
	}
	if len(hook3.events) != 1 {
		t.Error("Hook3 should receive event")
	}
	
	// Test adding another hook
	hook4 := &MockTraceHook{events: []graph.TraceEvent{}}
	multiHook.AddHook(hook4)
	
	span2 := &graph.TraceSpan{
		Event:    graph.TraceEventNodeEnd,
		NodeName: "test",
	}
	
	multiHook.OnEvent(ctx, span2)
	
	if len(hook4.events) != 1 {
		t.Error("Hook4 should receive event after being added")
	}
}

// MockTraceHook for testing
type MockTraceHook struct {
	events []graph.TraceEvent
}

func (m *MockTraceHook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	m.events = append(m.events, span.Event)
}

// Test helper functions
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "hello", true},
		{"HELLO WORLD", "hello", true},
		{"hello world", "HELLO", true},
		{"testing", "test", true},
		{"testing", "Test", true},
		{"testing", "xyz", false},
		{"", "test", false},
		{"test", "", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q): got %v, want %v", 
					tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// Test traced runnable
func TestTracedRunnable(t *testing.T) {
	// Create mock runnable
	mockRunnable := &MockRunnable{
		result: "test_result",
	}
	
	// Create mock hook
	mockHook := &MockTraceHook{
		events: []graph.TraceEvent{},
	}
	
	// Create traced runnable
	traced := NewTracedRunnable(mockRunnable, mockHook)
	
	// Execute
	ctx := context.Background()
	input := map[string]interface{}{
		"test": "input",
	}
	
	result, err := traced.Invoke(ctx, input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if result != "test_result" {
		t.Errorf("Result: got %v, want test_result", result)
	}
}

// MockRunnable for testing
type MockRunnable struct {
	result interface{}
	err    error
}

func (m *MockRunnable) Invoke(ctx context.Context, input interface{}) (interface{}, error) {
	return m.result, m.err
}

func (m *MockRunnable) Stream(ctx context.Context, input interface{}) (<-chan interface{}, <-chan error) {
	outputChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)
	
	if m.err != nil {
		errorChan <- m.err
	} else {
		outputChan <- m.result
	}
	
	close(outputChan)
	close(errorChan)
	
	return outputChan, errorChan
}

// Benchmark tests
func BenchmarkHookOnEvent(b *testing.B) {
	hook := NewHook()
	ctx := context.Background()
	
	span := &graph.TraceSpan{
		ID:        uuid.New().String(),
		Event:     graph.TraceEventNodeStart,
		NodeName:  "benchmark_node",
		StartTime: time.Now(),
		State:     map[string]interface{}{"key": "value"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hook.OnEvent(ctx, span)
	}
}

func BenchmarkFilteredHook(b *testing.B) {
	baseHook := NewHook()
	filter := EventFilter{
		IncludeEvents: []graph.TraceEvent{
			graph.TraceEventNodeStart,
			graph.TraceEventNodeEnd,
		},
		MinDuration: 10 * time.Millisecond,
	}
	
	filteredHook := NewFilteredHook(baseHook, filter)
	ctx := context.Background()
	
	span := &graph.TraceSpan{
		Event:     graph.TraceEventNodeEnd,
		Duration:  100 * time.Millisecond,
		NodeName:  "benchmark_node",
		StartTime: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filteredHook.OnEvent(ctx, span)
	}
}

func BenchmarkMultiHook(b *testing.B) {
	hook1 := NewHook()
	hook2 := NewHook()
	hook3 := NewHook()
	
	multiHook := NewMultiHook(hook1, hook2, hook3)
	ctx := context.Background()
	
	span := &graph.TraceSpan{
		Event:     graph.TraceEventNodeStart,
		NodeName:  "benchmark_node",
		StartTime: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		multiHook.OnEvent(ctx, span)
	}
}
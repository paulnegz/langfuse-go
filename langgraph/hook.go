package langgraph

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
	langfuse "github.com/paulnegz/langfuse-go"
	"github.com/paulnegz/langfuse-go/model"
	"github.com/tmc/langgraphgo/graph"
)

// Hook implements graph.TraceHook to send traces to Langfuse
type Hook struct {
	client       *langfuse.Langfuse
	enabled      bool
	traces       map[string]*model.Trace // Map graph span IDs to Langfuse traces
	observations map[string]string       // Map node span IDs to Langfuse observation IDs
	parents      map[string]string       // Map observation IDs to their parent IDs
	initialInput interface{}             // Store the initial workflow input for root span
	mu           sync.RWMutex
	ctx          context.Context
	config       *Config
}

// Config holds configuration options for the hook
type Config struct {
	// AutoFlush enables automatic flushing of traces at graph end
	AutoFlush bool
	// DefaultMetadata is added to all traces
	DefaultMetadata map[string]interface{}
	// TraceName allows customizing the trace name
	TraceName string
	// SessionID for grouping related traces
	SessionID string
	// UserID for identifying the user
	UserID string
	// Tags to add to traces
	Tags []string
}

// Option is a functional option for configuring the hook
type Option func(*Config)

// WithAutoFlush enables automatic flushing at graph end
func WithAutoFlush(enabled bool) Option {
	return func(c *Config) {
		c.AutoFlush = enabled
	}
}

// WithMetadata adds default metadata to all traces
func WithMetadata(metadata map[string]interface{}) Option {
	return func(c *Config) {
		c.DefaultMetadata = metadata
	}
}

// WithTraceName sets a custom trace name
func WithTraceName(name string) Option {
	return func(c *Config) {
		c.TraceName = name
	}
}

// WithSessionID sets the session ID for traces
func WithSessionID(id string) Option {
	return func(c *Config) {
		c.SessionID = id
	}
}

// WithUserID sets the user ID for traces
func WithUserID(id string) Option {
	return func(c *Config) {
		c.UserID = id
	}
}

// WithTags adds tags to traces
func WithTags(tags []string) Option {
	return func(c *Config) {
		c.Tags = tags
	}
}

// NewHook creates a new Langfuse trace hook
func NewHook(opts ...Option) *Hook {
	config := &Config{
		AutoFlush:       true,
		DefaultMetadata: make(map[string]interface{}),
		TraceName:       "langgraph_workflow",
		Tags:            []string{"golang", "langgraph"},
	}

	for _, opt := range opts {
		opt(config)
	}

	// Check if Langfuse is configured
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")

	if publicKey == "" || secretKey == "" {
		log.Println("Langfuse not configured, tracing disabled")
		return &Hook{
			enabled: false,
			config:  config,
		}
	}

	// Create context and client
	ctx := context.Background()
	client := langfuse.New(ctx)

	return &Hook{
		client:       client,
		enabled:      true,
		traces:       make(map[string]*model.Trace),
		observations: make(map[string]string),
		parents:      make(map[string]string),
		ctx:          ctx,
		config:       config,
		mu:           sync.RWMutex{},
	}
}

// NewHookWithClient creates a new hook with an existing Langfuse client
func NewHookWithClient(client *langfuse.Langfuse, opts ...Option) *Hook {
	config := &Config{
		AutoFlush:       true,
		DefaultMetadata: make(map[string]interface{}),
		TraceName:       "langgraph_workflow",
		Tags:            []string{"golang", "langgraph"},
	}

	for _, opt := range opts {
		opt(config)
	}

	return &Hook{
		client:       client,
		enabled:      true,
		traces:       make(map[string]*model.Trace),
		observations: make(map[string]string),
		parents:      make(map[string]string),
		ctx:          context.Background(),
		config:       config,
		mu:           sync.RWMutex{},
	}
}

// SetInitialInput stores the initial workflow input for use in traces
func (h *Hook) SetInitialInput(input interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.initialInput = input
}

// OnEvent handles trace events and sends them to Langfuse
func (h *Hook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	if !h.enabled {
		return
	}

	switch span.Event {
	case graph.TraceEventGraphStart:
		h.handleGraphStart(ctx, span)
	case graph.TraceEventGraphEnd:
		h.handleGraphEnd(ctx, span)
	case graph.TraceEventNodeStart:
		h.handleNodeStart(ctx, span)
	case graph.TraceEventNodeEnd, graph.TraceEventNodeError:
		h.handleNodeEnd(ctx, span)
	case graph.TraceEventEdgeTraversal:
		// Skip edge events for now
		return
	}
}

// handleGraphStart creates a new Langfuse trace
func (h *Hook) handleGraphStart(ctx context.Context, span *graph.TraceSpan) {
	h.mu.Lock()
	defer h.mu.Unlock()

	traceID := uuid.New().String()
	now := span.StartTime

	// Merge metadata
	metadata := make(map[string]interface{})
	for k, v := range h.config.DefaultMetadata {
		metadata[k] = v
	}
	for k, v := range span.Metadata {
		metadata[k] = v
	}
	metadata["graph_span_id"] = span.ID
	metadata["sdk"] = "langfuse-go/langgraph"
	metadata["sdk_version"] = "1.0.0"

	// Use configuration or metadata values
	userID := h.config.UserID
	sessionID := h.config.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("graph_%s", traceID)
	}
	if uid, ok := metadata["user_id"].(string); ok && userID == "" {
		userID = uid
	}
	if sid, ok := metadata["session_id"].(string); ok && sessionID == "" {
		sessionID = sid
	}

	trace := &model.Trace{
		ID:        traceID,
		Timestamp: &now,
		Name:      h.config.TraceName,
		UserID:    userID,
		SessionID: sessionID,
		Input:     h.initialInput,
		Metadata:  metadata,
		Tags:      h.config.Tags,
	}

	// Send trace to Langfuse
	_, err := h.client.Trace(trace)
	if err != nil {
		log.Printf("Failed to create Langfuse trace: %v", err)
		return
	}

	// Store trace for later reference
	h.traces[span.ID] = trace

	// Create workflow root span
	rootSpanID := uuid.New().String()
	rootSpan := &model.Span{
		ID:        rootSpanID,
		TraceID:   traceID,
		Name:      h.config.TraceName,
		StartTime: &now,
		Input:     h.initialInput,
		Metadata: map[string]interface{}{
			"graph_span_id": span.ID,
			"sdk":           "langfuse-go/langgraph",
			"sdk_version":   "1.0.0",
		},
	}

	createdRootSpan, err := h.client.Span(rootSpan, nil)
	if err != nil {
		log.Printf("Failed to create root span: %v", err)
	} else if createdRootSpan != nil && createdRootSpan.ID != "" {
		rootSpanID = createdRootSpan.ID
	}

	// Store as parent for all top-level operations
	h.observations["langgraph_wrapper"] = rootSpanID
	h.observations["default_parent"] = rootSpanID
	h.observations[span.ID] = rootSpanID
	h.parents[rootSpanID] = ""
}

// handleGraphEnd updates the trace with final information
func (h *Hook) handleGraphEnd(ctx context.Context, span *graph.TraceSpan) {
	h.mu.Lock()
	defer h.mu.Unlock()

	trace, ok := h.traces[span.ID]
	if !ok {
		return
	}

	// Update trace with end time and duration
	endTime := span.EndTime

	// Update metadata
	if metadata, ok := trace.Metadata.(map[string]interface{}); ok {
		metadata["duration_ms"] = span.Duration.Milliseconds()
		metadata["status"] = "completed"
		if span.Error != nil {
			metadata["error"] = span.Error.Error()
			metadata["status"] = "error"
		}
		trace.Metadata = metadata
	}

	// Update the trace
	_, err := h.client.Trace(&model.Trace{
		ID:        trace.ID,
		Timestamp: &endTime,
		Output:    span.State,
		Metadata:  trace.Metadata,
	})
	if err != nil {
		log.Printf("Failed to update Langfuse trace: %v", err)
	}

	// Update root span
	if rootSpanID, ok := h.observations[span.ID]; ok {
		rootSpan := &model.Span{
			ID:      rootSpanID,
			TraceID: trace.ID,
			Name:    h.config.TraceName,
			EndTime: &endTime,
			Output:  span.State,
		}
		_, err := h.client.Span(rootSpan, nil)
		if err != nil {
			log.Printf("Failed to update root span: %v", err)
		}
	}

	// Auto-flush if configured
	if h.config.AutoFlush {
		h.client.Flush(h.ctx)
	}
}

// handleNodeStart creates a span for node execution
func (h *Hook) handleNodeStart(ctx context.Context, span *graph.TraceSpan) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find parent trace
	var traceID string
	if span.ParentID != "" {
		if trace, ok := h.traces[span.ParentID]; ok {
			traceID = trace.ID
		}
	} else {
		// Find the current trace
		for _, trace := range h.traces {
			traceID = trace.ID
			break
		}
	}

	if traceID == "" {
		return
	}

	spanID := uuid.New().String()
	startTime := span.StartTime

	// Check if this is an AI operation
	isAINode := h.isAIOperation(span.NodeName)

	if isAINode {
		// Create generation for AI operations
		generation := &model.Generation{
			ID:        spanID,
			TraceID:   traceID,
			Name:      fmt.Sprintf("%s_generation", span.NodeName),
			StartTime: &startTime,
			Model:     h.extractModel(span),
			Input:     span.State,
			Metadata: map[string]interface{}{
				"node_name":     span.NodeName,
				"graph_span_id": span.ID,
			},
			ModelParameters: h.extractModelParams(span),
		}

		// Find parent observation
		var parentObsID *string
		if defaultParent, ok := h.observations["default_parent"]; ok {
			parentObsID = &defaultParent
		}

		createdGen, err := h.client.Generation(generation, parentObsID)
		if err != nil {
			log.Printf("Failed to create generation: %v", err)
			return
		}
		if createdGen != nil && createdGen.ID != "" {
			spanID = createdGen.ID
		}
		if parentObsID != nil {
			h.parents[spanID] = *parentObsID
		}
	} else {
		// Create span for non-AI operations
		langfuseSpan := &model.Span{
			ID:        spanID,
			TraceID:   traceID,
			Name:      span.NodeName,
			StartTime: &startTime,
			Input:     span.State,
			Metadata: map[string]interface{}{
				"node_name":     span.NodeName,
				"graph_span_id": span.ID,
			},
		}

		// Find parent observation
		var parentObsID *string
		if defaultParent, ok := h.observations["default_parent"]; ok {
			parentObsID = &defaultParent
		}

		createdSpan, err := h.client.Span(langfuseSpan, parentObsID)
		if err != nil {
			log.Printf("Failed to create span: %v", err)
			return
		}
		if createdSpan != nil && createdSpan.ID != "" {
			spanID = createdSpan.ID
		}
		if parentObsID != nil {
			h.parents[spanID] = *parentObsID
		}
	}

	// Store observation ID
	h.observations[span.ID] = spanID
}

// handleNodeEnd updates the span/generation with completion information
func (h *Hook) handleNodeEnd(ctx context.Context, span *graph.TraceSpan) {
	h.mu.Lock()
	defer h.mu.Unlock()

	obsID, ok := h.observations[span.ID]
	if !ok {
		return
	}

	// Find parent trace
	var traceID string
	if span.ParentID != "" {
		if trace, ok := h.traces[span.ParentID]; ok {
			traceID = trace.ID
		}
	}

	if traceID == "" {
		return
	}

	endTime := span.EndTime
	metadata := map[string]interface{}{
		"duration_ms": span.Duration.Milliseconds(),
		"node_name":   span.NodeName,
	}

	if span.Error != nil {
		metadata["error"] = span.Error.Error()
		metadata["status"] = "error"
	} else {
		metadata["status"] = "completed"
	}

	// Check if this is an AI operation
	isAINode := h.isAIOperation(span.NodeName)

	if isAINode {
		// Update generation
		generation := &model.Generation{
			ID:       obsID,
			TraceID:  traceID,
			Name:     fmt.Sprintf("%s_generation", span.NodeName),
			EndTime:  &endTime,
			Output:   span.State,
			Metadata: metadata,
			Usage:    h.extractUsage(span),
		}

		var parentObsID *string
		if parentID, ok := h.parents[obsID]; ok && parentID != "" {
			parentObsID = &parentID
		}

		_, err := h.client.Generation(generation, parentObsID)
		if err != nil {
			log.Printf("Failed to update generation: %v", err)
		}
	} else {
		// Update span
		langfuseSpan := &model.Span{
			ID:       obsID,
			TraceID:  traceID,
			Name:     span.NodeName,
			EndTime:  &endTime,
			Output:   span.State,
			Metadata: metadata,
		}

		var parentObsID *string
		if parentID, ok := h.parents[obsID]; ok && parentID != "" {
			parentObsID = &parentID
		}

		_, err := h.client.Span(langfuseSpan, parentObsID)
		if err != nil {
			log.Printf("Failed to update span: %v", err)
		}
	}
}

// Flush ensures all pending events are sent
func (h *Hook) Flush() {
	if !h.enabled {
		return
	}
	h.client.Flush(h.ctx)
}

// Helper methods

func (h *Hook) isAIOperation(nodeName string) bool {
	// Detect AI operations based on node name patterns
	aiPatterns := []string{
		"ai", "llm", "generate", "completion", "chat",
		"gpt", "claude", "gemini", "openai",
	}
	
	for _, pattern := range aiPatterns {
		if containsIgnoreCase(nodeName, pattern) {
			return true
		}
	}
	return false
}

func (h *Hook) extractModel(span *graph.TraceSpan) string {
	// Extract model from metadata if available
	if span.Metadata != nil {
		if model, ok := span.Metadata["model"].(string); ok {
			return model
		}
	}
	// Default model names based on patterns
	if containsIgnoreCase(span.NodeName, "gpt") {
		return "gpt-3.5-turbo"
	}
	if containsIgnoreCase(span.NodeName, "claude") {
		return "claude-3-sonnet"
	}
	if containsIgnoreCase(span.NodeName, "gemini") {
		return "gemini-pro"
	}
	return "unknown"
}

func (h *Hook) extractModelParams(span *graph.TraceSpan) map[string]interface{} {
	params := make(map[string]interface{})
	
	// Default parameters
	params["temperature"] = 0.7
	params["max_tokens"] = 2048
	
	// Override with metadata if available
	if span.Metadata != nil {
		if temp, ok := span.Metadata["temperature"]; ok {
			params["temperature"] = temp
		}
		if maxTokens, ok := span.Metadata["max_tokens"]; ok {
			params["max_tokens"] = maxTokens
		}
	}
	
	return params
}

func (h *Hook) extractUsage(span *graph.TraceSpan) model.Usage {
	// Extract usage from metadata if available
	if span.Metadata != nil {
		if usage, ok := span.Metadata["usage"].(map[string]interface{}); ok {
			input, _ := usage["input"].(int)
			output, _ := usage["output"].(int)
			return model.Usage{
				Input:  input,
				Output: output,
				Total:  input + output,
			}
		}
	}
	
	// Return estimated usage
	return model.Usage{
		Input:  100,
		Output: 200,
		Total:  300,
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		containsString(toLowerCase(s), toLowerCase(substr)))
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
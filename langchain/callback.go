package langchain

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	langfuse "github.com/paulnegz/langfuse-go"
	"github.com/paulnegz/langfuse-go/model"
)

// CallbackHandler implements LangChain-compatible callbacks for Langfuse
// This matches Python's langfuse.langchain.CallbackHandler
type CallbackHandler struct {
	client       *langfuse.Langfuse
	traces       map[string]*model.Trace
	observations map[string]interface{} // Can be Span or Generation
	traceName    string
	userID       string
	sessionID    string
	metadata     map[string]interface{}
	mu           sync.RWMutex
	ctx          context.Context
}

// NewCallbackHandler creates a new Langfuse callback handler
func NewCallbackHandler() *CallbackHandler {
	ctx := context.Background()
	client := langfuse.New(ctx)

	return &CallbackHandler{
		client:       client,
		traces:       make(map[string]*model.Trace),
		observations: make(map[string]interface{}),
		ctx:          ctx,
		mu:           sync.RWMutex{},
	}
}

// SetTraceParams sets trace parameters (matches Python's handler.trace_name = "...")
func (h *CallbackHandler) SetTraceParams(traceName, userID, sessionID string, metadata map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.traceName = traceName
	h.userID = userID
	h.sessionID = sessionID
	h.metadata = metadata
}

// OnChainStart is called when a chain/graph starts
func (h *CallbackHandler) OnChainStart(ctx context.Context, serialized map[string]interface{}, inputs map[string]interface{}, runID string, parentRunID *string, tags []string, metadata map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	name := h.traceName
	if name == "" {
		if n, nameExists := serialized["name"].(string); nameExists {
			name = n
		} else {
			name = "chain"
		}
	}

	now := time.Now()

	if parentRunID == nil {
		// Root trace
		trace := &model.Trace{
			ID:        runID,
			Timestamp: &now,
			Name:      name,
			UserID:    h.userID,
			SessionID: h.sessionID,
			Input:     inputs,
			Metadata:  h.mergeMetadata(metadata),
			Tags:      tags,
		}

		if _, err := h.client.Trace(trace); err != nil {
			_, _ = fmt.Printf("Failed to create trace: %v\n", err)
		}

		h.traces[runID] = trace
	} else {
		// Child span
		parentObsID := *parentRunID
		span := &model.Span{
			ID:                  runID,
			TraceID:             h.findTraceID(*parentRunID),
			ParentObservationID: parentObsID,
			Name:                name,
			StartTime:           &now,
			Input:               inputs,
			Metadata:            metadata,
		}

		if _, err := h.client.Span(span, nil); err != nil {
			_, _ = fmt.Printf("Failed to create span: %v\n", err)
		}

		h.observations[runID] = span
	}
}

// OnChainEnd is called when a chain/graph ends
func (h *CallbackHandler) OnChainEnd(ctx context.Context, outputs map[string]interface{}, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if trace, traceExists := h.traces[runID]; traceExists {
		// Update trace
		trace.Output = outputs
		if _, err := h.client.Trace(&model.Trace{
			ID:     runID,
			Output: outputs,
		}); err != nil {
			_, _ = fmt.Printf("Failed to update trace: %v\n", err)
		}
	} else if obs, obsExists := h.observations[runID]; obsExists {
		// Update span
		if span, isSpan := obs.(*model.Span); isSpan {
			span.EndTime = &now
			span.Output = outputs
			if _, err := h.client.Span(&model.Span{
				ID:      runID,
				EndTime: &now,
				Output:  outputs,
			}, nil); err != nil {
				_, _ = fmt.Printf("Failed to update span: %v\n", err)
			}
		}
	}
}

// OnChainError is called when a chain/graph errors
func (h *CallbackHandler) OnChainError(ctx context.Context, err error, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	errorMsg := err.Error()
	metadata := map[string]interface{}{
		"error":  errorMsg,
		"status": "error",
	}

	if trace, exists := h.traces[runID]; exists {
		// Update trace with error
		trace.Output = map[string]interface{}{"error": errorMsg}
		trace.Metadata = h.mergeMetadata(metadata)
		if _, updateErr := h.client.Trace(&model.Trace{
			ID:       runID,
			Output:   trace.Output,
			Metadata: trace.Metadata,
		}); updateErr != nil {
			_, _ = fmt.Printf("Failed to update trace with error: %v\n", updateErr)
		}
	}
}

// OnLLMStart is called when an LLM call starts
func (h *CallbackHandler) OnLLMStart(ctx context.Context, serialized map[string]interface{}, prompts []string, runID string, parentRunID *string, tags []string, metadata map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	modelName := "unknown"
	if modelStr, exists := serialized["model"].(string); exists {
		modelName = modelStr
	} else if metaModelStr, metaExists := metadata["model"].(string); metaExists {
		modelName = metaModelStr
	}

	parentObsID := ""
	if parentRunID != nil {
		parentObsID = *parentRunID
	}
	generation := &model.Generation{
		ID:                  runID,
		TraceID:             h.findTraceID(runID),
		ParentObservationID: parentObsID,
		Name:                fmt.Sprintf("%s-generation", modelName),
		Model:               modelName,
		StartTime:           &now,
		Input:               prompts,
		Metadata:            metadata,
	}

	if _, err := h.client.Generation(generation, nil); err != nil {
		_, _ = fmt.Printf("Failed to create generation: %v\n", err)
	}

	h.observations[runID] = generation
}

// OnLLMEnd is called when an LLM call ends
func (h *CallbackHandler) OnLLMEnd(ctx context.Context, response interface{}, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if obs, exists := h.observations[runID]; exists {
		if gen, isGen := obs.(*model.Generation); isGen {
			gen.EndTime = &now
			gen.Output = response

			// Try to extract token usage if available
			if respMap, isMap := response.(map[string]interface{}); isMap {
				if usage, hasUsage := respMap["usage"].(map[string]interface{}); hasUsage {
					if total, hasTotal := usage["total_tokens"].(int); hasTotal {
						gen.Usage = model.Usage{
							TotalTokens: total,
						}
					}
				}
			}

			if _, err := h.client.Generation(&model.Generation{
				ID:      runID,
				EndTime: &now,
				Output:  response,
				Usage:   gen.Usage,
			}, nil); err != nil {
				_, _ = fmt.Printf("Failed to update generation: %v\n", err)
			}
		}
	}
}

// OnLLMError is called when an LLM call errors
func (h *CallbackHandler) OnLLMError(ctx context.Context, err error, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if obs, exists := h.observations[runID]; exists {
		if gen, isGen := obs.(*model.Generation); isGen {
			gen.EndTime = &now
			gen.StatusMessage = err.Error()

			if _, updateErr := h.client.Generation(&model.Generation{
				ID:            runID,
				EndTime:       &now,
				StatusMessage: err.Error(),
			}, nil); updateErr != nil {
				_, _ = fmt.Printf("Failed to update generation with error: %v\n", updateErr)
			}
		}
	}
}

// OnToolStart is called when a tool call starts
func (h *CallbackHandler) OnToolStart(ctx context.Context, serialized map[string]interface{}, inputStr string, runID string, parentRunID *string, tags []string, metadata map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	toolName := "tool"
	if nameStr, exists := serialized["name"].(string); exists {
		toolName = nameStr
	}

	parentObsIDTool := ""
	if parentRunID != nil {
		parentObsIDTool = *parentRunID
	}
	span := &model.Span{
		ID:                  runID,
		TraceID:             h.findTraceID(runID),
		ParentObservationID: parentObsIDTool,
		Name:                toolName,
		StartTime:           &now,
		Input:               inputStr,
		Metadata:            metadata,
	}

	if _, err := h.client.Span(span, nil); err != nil {
		_, _ = fmt.Printf("Failed to create tool span: %v\n", err)
	}

	h.observations[runID] = span
}

// OnToolEnd is called when a tool call ends
func (h *CallbackHandler) OnToolEnd(ctx context.Context, output string, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if obs, exists := h.observations[runID]; exists {
		if span, isSpan := obs.(*model.Span); isSpan {
			span.EndTime = &now
			span.Output = output

			if _, err := h.client.Span(&model.Span{
				ID:      runID,
				EndTime: &now,
				Output:  output,
			}, nil); err != nil {
				_, _ = fmt.Printf("Failed to update tool span: %v\n", err)
			}
		}
	}
}

// OnToolError is called when a tool call errors
func (h *CallbackHandler) OnToolError(ctx context.Context, err error, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	if obs, exists := h.observations[runID]; exists {
		if span, isSpan := obs.(*model.Span); isSpan {
			span.EndTime = &now
			span.StatusMessage = err.Error()

			if _, updateErr := h.client.Span(&model.Span{
				ID:            runID,
				EndTime:       &now,
				StatusMessage: err.Error(),
			}, nil); updateErr != nil {
				_, _ = fmt.Printf("Failed to update tool span with error: %v\n", updateErr)
			}
		}
	}
}

// OnRetrieverStart is called when a retriever starts
func (h *CallbackHandler) OnRetrieverStart(ctx context.Context, serialized map[string]interface{}, query string, runID string, parentRunID *string, tags []string, metadata map[string]interface{}) {
	// Implement if needed for retriever tracing
	h.OnToolStart(ctx, serialized, query, runID, parentRunID, tags, metadata)
}

// OnRetrieverEnd is called when a retriever ends
func (h *CallbackHandler) OnRetrieverEnd(ctx context.Context, documents []interface{}, runID string) {
	// Implement if needed for retriever tracing
	output := fmt.Sprintf("Retrieved %d documents", len(documents))
	h.OnToolEnd(ctx, output, runID)
}

// OnRetrieverError is called when a retriever errors
func (h *CallbackHandler) OnRetrieverError(ctx context.Context, err error, runID string) {
	// Implement if needed for retriever tracing
	h.OnToolError(ctx, err, runID)
}

// Helper methods

func (h *CallbackHandler) findTraceID(runID string) string {
	// First check if this runID is a trace
	if trace, found := h.traces[runID]; found {
		return trace.ID
	}

	// Otherwise, look for the parent trace
	for traceID := range h.traces {
		return traceID // Return first trace (should only be one root)
	}

	// Fallback: generate new trace ID
	return uuid.New().String()
}

func (h *CallbackHandler) mergeMetadata(additional map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Add base metadata
	for k, v := range h.metadata {
		result[k] = v
	}

	// Add additional metadata
	for k, v := range additional {
		result[k] = v
	}

	// Add environment info
	result["sdk"] = "langfuse-go"
	result["environment"] = os.Getenv("ENVIRONMENT")

	return result
}

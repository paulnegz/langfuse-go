package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/paulnegz/langfuse-go/langgraph"
	"github.com/tmc/langgraphgo/graph"
)

// Example: Advanced LangGraph workflow with conditional routing and AI operations
func main() {
	// Set up Langfuse credentials
	os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

	// Create advanced workflow with conditional logic
	workflow := createAdvancedWorkflow()

	// Use builder pattern for configuration
	hook := langgraph.NewBuilder().
		WithTraceName("advanced_ai_workflow").
		WithSessionID(fmt.Sprintf("session-%d", time.Now().Unix())).
		WithUserID("power-user-456").
		WithMetadata(map[string]interface{}{
			"example":     "advanced",
			"complexity":  "high",
			"ai_enabled":  true,
			"environment": "production",
		}).
		WithTags("advanced", "ai", "conditional").
		WithAutoFlush(true).
		Build()

	// Create tracer with hook
	tracer := graph.NewTracer()
	tracer.AddHook(hook)

	// Compile and create traced runnable
	compiled := workflow.Compile()
	tracedWorkflow := graph.NewTracedRunnable(compiled, tracer)

	// Prepare complex input
	input := map[string]interface{}{
		"query":      "Explain quantum computing",
		"language":   "en",
		"max_length": 500,
		"style":      "technical",
		"metadata": map[string]interface{}{
			"request_id": fmt.Sprintf("req-%d", rand.Int63()),
			"timestamp":  time.Now().Unix(),
		},
	}

	// Set initial input
	hook.SetInitialInput(input)

	// Execute workflow
	ctx := context.Background()
	result, err := tracedWorkflow.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("Advanced workflow failed: %v", err)
	}

	fmt.Printf("Advanced workflow completed!\n")
	fmt.Printf("Result: %+v\n", result)
}

// createAdvancedWorkflow creates a complex workflow with multiple paths
func createAdvancedWorkflow() *graph.StateGraph {
	type AIResponse struct {
		Content    string
		Model      string
		TokenCount int
		Duration   time.Duration
	}

	type WorkflowState struct {
		Query       string
		Language    string
		MaxLength   int
		Style       string
		Complexity  string
		Response    AIResponse
		Cache       map[string]interface{}
		Errors      []string
		Metadata    map[string]interface{}
	}

	workflow := graph.NewStateGraph(WorkflowState{})

	// Entry point: Validate and classify query
	workflow.AddNode("classify_query", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		// Simulate query classification
		if len(state.Query) < 10 {
			state.Complexity = "simple"
		} else if len(state.Query) < 50 {
			state.Complexity = "moderate"
		} else {
			state.Complexity = "complex"
		}
		
		log.Printf("Query classified as: %s", state.Complexity)
		return state, nil
	})

	// Check cache for existing responses
	workflow.AddNode("check_cache", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		if state.Cache == nil {
			state.Cache = make(map[string]interface{})
		}
		
		cacheKey := fmt.Sprintf("%s_%s_%s", state.Query, state.Language, state.Style)
		if cached, exists := state.Cache[cacheKey]; exists {
			log.Println("Cache hit!")
			if response, ok := cached.(AIResponse); ok {
				state.Response = response
			}
		}
		return state, nil
	})

	// Simple AI operation for basic queries
	workflow.AddNode("simple_ai_generation", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		start := time.Now()
		
		// Simulate simple AI generation
		time.Sleep(100 * time.Millisecond)
		
		state.Response = AIResponse{
			Content:    fmt.Sprintf("Simple response to: %s", state.Query),
			Model:      "gpt-3.5-turbo",
			TokenCount: 50 + rand.Intn(50),
			Duration:   time.Since(start),
		}
		
		// Add to metadata for tracing
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["model"] = state.Response.Model
		state.Metadata["usage"] = map[string]interface{}{
			"input":  25,
			"output": state.Response.TokenCount,
		}
		
		return state, nil
	})

	// Complex AI operation for advanced queries
	workflow.AddNode("complex_ai_generation", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		start := time.Now()
		
		// Simulate complex AI generation with multiple steps
		time.Sleep(300 * time.Millisecond)
		
		state.Response = AIResponse{
			Content: fmt.Sprintf("Advanced %s explanation of '%s': [Detailed technical content would go here]",
				state.Style, state.Query),
			Model:      "gpt-4",
			TokenCount: 200 + rand.Intn(300),
			Duration:   time.Since(start),
		}
		
		// Add to metadata
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["model"] = state.Response.Model
		state.Metadata["temperature"] = 0.7
		state.Metadata["max_tokens"] = state.MaxLength
		state.Metadata["usage"] = map[string]interface{}{
			"input":  100,
			"output": state.Response.TokenCount,
		}
		
		return state, nil
	})

	// Post-processing and formatting
	workflow.AddNode("format_response", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		// Apply formatting based on style
		switch state.Style {
		case "technical":
			state.Response.Content = fmt.Sprintf("## Technical Analysis\n\n%s\n\n*Model: %s | Tokens: %d*",
				state.Response.Content, state.Response.Model, state.Response.TokenCount)
		case "simple":
			state.Response.Content = fmt.Sprintf("Here's a simple explanation:\n\n%s",
				state.Response.Content)
		default:
			state.Response.Content = fmt.Sprintf("%s\n\n---\nGenerated in %v",
				state.Response.Content, state.Response.Duration)
		}
		return state, nil
	})

	// Save to cache
	workflow.AddNode("save_to_cache", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		if state.Cache == nil {
			state.Cache = make(map[string]interface{})
		}
		
		cacheKey := fmt.Sprintf("%s_%s_%s", state.Query, state.Language, state.Style)
		state.Cache[cacheKey] = state.Response
		
		log.Printf("Response cached with key: %s", cacheKey)
		return state, nil
	})

	// Error handling node
	workflow.AddNode("handle_error", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		state.Response = AIResponse{
			Content: "An error occurred during processing. Please try again.",
			Model:   "fallback",
		}
		state.Errors = append(state.Errors, "Processing failed")
		return state, nil
	})

	// Define conditional routing
	workflow.SetEntryPoint("classify_query")
	workflow.AddEdge("classify_query", "check_cache")
	
	// Conditional routing based on cache and complexity
	workflow.AddConditionalEdges("check_cache",
		func(ctx context.Context, state WorkflowState) string {
			// If cached, skip to formatting
			if state.Response.Content != "" {
				return "format_response"
			}
			// Route based on complexity
			if state.Complexity == "complex" {
				return "complex_ai_generation"
			}
			return "simple_ai_generation"
		},
		map[string]string{
			"simple_ai_generation":  "simple_ai_generation",
			"complex_ai_generation": "complex_ai_generation",
			"format_response":       "format_response",
		},
	)
	
	// Both AI nodes lead to formatting
	workflow.AddEdge("simple_ai_generation", "format_response")
	workflow.AddEdge("complex_ai_generation", "format_response")
	
	// Format leads to caching
	workflow.AddEdge("format_response", "save_to_cache")
	
	// Cache leads to END
	workflow.AddEdge("save_to_cache", graph.END)
	
	// Error handling can lead to END
	workflow.AddEdge("handle_error", graph.END)

	return workflow
}
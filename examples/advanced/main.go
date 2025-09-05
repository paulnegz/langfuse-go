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
	_ = os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	_ = os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

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
	compiled, err := workflow.Compile()
	if err != nil {
		log.Fatalf("Failed to compile workflow: %v", err)
	}
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
		Query      string
		Language   string
		MaxLength  int
		Style      string
		Complexity string
		Response   AIResponse
		Cache      map[string]interface{}
		Errors     []string
		Metadata   map[string]interface{}
	}

	workflow := graph.NewStateGraph()

	// Entry point: Validate and classify query
	workflow.AddNode("classify_query", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		// Simulate query classification
		if len(ws.Query) < 10 {
			ws.Complexity = "simple"
		} else if len(ws.Query) < 50 {
			ws.Complexity = "moderate"
		} else {
			ws.Complexity = "complex"
		}

		log.Printf("Query classified as: %s", ws.Complexity)
		return ws, nil
	})

	// Check cache for existing responses
	workflow.AddNode("check_cache", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		if ws.Cache == nil {
			ws.Cache = make(map[string]interface{})
		}

		cacheKey := fmt.Sprintf("%s_%s_%s", ws.Query, ws.Language, ws.Style)
		if cached, exists := ws.Cache[cacheKey]; exists {
			log.Println("Cache hit!")
			if response, ok := cached.(AIResponse); ok {
				ws.Response = response
			}
		}
		return ws, nil
	})

	// Simple AI operation for basic queries
	workflow.AddNode("simple_ai_generation", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		start := time.Now()

		// Simulate simple AI generation
		time.Sleep(100 * time.Millisecond)

		ws.Response = AIResponse{
			Content:    fmt.Sprintf("Simple response to: %s", ws.Query),
			Model:      "gpt-3.5-turbo",
			TokenCount: 50 + rand.Intn(50),
			Duration:   time.Since(start),
		}

		// Add to metadata for tracing
		if ws.Metadata == nil {
			ws.Metadata = make(map[string]interface{})
		}
		ws.Metadata["model"] = ws.Response.Model
		ws.Metadata["usage"] = map[string]interface{}{
			"input":  25,
			"output": ws.Response.TokenCount,
		}

		return ws, nil
	})

	// Complex AI operation for advanced queries
	workflow.AddNode("complex_ai_generation", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		start := time.Now()

		// Simulate complex AI generation with multiple steps
		time.Sleep(300 * time.Millisecond)

		ws.Response = AIResponse{
			Content: fmt.Sprintf("Advanced %s explanation of '%s': [Detailed technical content would go here]",
				ws.Style, ws.Query),
			Model:      "gpt-4",
			TokenCount: 200 + rand.Intn(300),
			Duration:   time.Since(start),
		}

		// Add to metadata
		if ws.Metadata == nil {
			ws.Metadata = make(map[string]interface{})
		}
		ws.Metadata["model"] = ws.Response.Model
		ws.Metadata["temperature"] = 0.7
		ws.Metadata["max_tokens"] = ws.MaxLength
		ws.Metadata["usage"] = map[string]interface{}{
			"input":  100,
			"output": ws.Response.TokenCount,
		}

		return ws, nil
	})

	// Post-processing and formatting
	workflow.AddNode("format_response", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		// Apply formatting based on style
		switch ws.Style {
		case "technical":
			ws.Response.Content = fmt.Sprintf("## Technical Analysis\n\n%s\n\n*Model: %s | Tokens: %d*",
				ws.Response.Content, ws.Response.Model, ws.Response.TokenCount)
		case "simple":
			ws.Response.Content = fmt.Sprintf("Here's a simple explanation:\n\n%s",
				ws.Response.Content)
		default:
			ws.Response.Content = fmt.Sprintf("%s\n\n---\nGenerated in %v",
				ws.Response.Content, ws.Response.Duration)
		}
		return ws, nil
	})

	// Save to cache
	workflow.AddNode("save_to_cache", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		if ws.Cache == nil {
			ws.Cache = make(map[string]interface{})
		}

		cacheKey := fmt.Sprintf("%s_%s_%s", ws.Query, ws.Language, ws.Style)
		ws.Cache[cacheKey] = ws.Response

		log.Printf("Response cached with key: %s", cacheKey)
		return ws, nil
	})

	// Error handling node
	workflow.AddNode("handle_error", func(ctx context.Context, state interface{}) (interface{}, error) {
		ws := state.(WorkflowState)
		ws.Response = AIResponse{
			Content: "An error occurred during processing. Please try again.",
			Model:   "fallback",
		}
		ws.Errors = append(ws.Errors, "Processing failed")
		return ws, nil
	})

	// Define conditional routing
	workflow.SetEntryPoint("classify_query")
	workflow.AddEdge("classify_query", "check_cache")

	// Conditional routing based on cache and complexity
	workflow.AddConditionalEdges("check_cache",
		func(ctx context.Context, state interface{}) string {
			ws := state.(WorkflowState)
			// If cached, skip to formatting
			if ws.Response.Content != "" {
				return "format_response"
			}
			// Route based on complexity
			if ws.Complexity == "complex" {
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

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/paulnegz/langfuse-go/langgraph"
	"github.com/tmc/langgraphgo/graph"
)

// Example: Basic LangGraph workflow with Langfuse tracing
func main() {
	// Set up Langfuse credentials
	_ = os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	_ = os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

	// Create a simple workflow
	workflow := createWorkflow()

	// Create Langfuse hook with configuration
	hook := langgraph.NewHook(
		langgraph.WithTraceName("basic_workflow"),
		langgraph.WithSessionID("session-001"),
		langgraph.WithUserID("user-123"),
		langgraph.WithMetadata(map[string]interface{}{
			"example": "basic",
			"version": "1.0.0",
		}),
		langgraph.WithTags([]string{"example", "basic"}),
	)

	// Create tracer and add hook
	tracer := graph.NewTracer()
	tracer.AddHook(hook)

	// Compile and create traced runnable
	compiled := workflow.Compile()
	tracedWorkflow := graph.NewTracedRunnable(compiled, tracer)

	// Prepare input
	input := map[string]interface{}{
		"message": "Hello, World!",
		"count":   3,
	}

	// Set initial input for proper trace context
	hook.SetInitialInput(input)

	// Execute workflow with tracing
	ctx := context.Background()
	result, err := tracedWorkflow.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	fmt.Printf("Workflow result: %v\n", result)

	// Ensure traces are flushed
	hook.Flush()
	fmt.Println("Traces sent to Langfuse!")
}

// createWorkflow creates a simple multi-step workflow
func createWorkflow() *graph.StateGraph {
	// Define the state structure
	type WorkflowState struct {
		Message string
		Count   int
		Steps   []string
	}

	// Create new state graph
	workflow := graph.NewStateGraph(WorkflowState{})

	// Add nodes
	workflow.AddNode("validate_input", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		state.Steps = append(state.Steps, "validated")
		if state.Message == "" {
			return state, fmt.Errorf("message cannot be empty")
		}
		return state, nil
	})

	workflow.AddNode("process_message", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		state.Steps = append(state.Steps, "processed")
		// Simple processing: repeat message
		for i := 0; i < state.Count; i++ {
			state.Message += "!"
		}
		return state, nil
	})

	workflow.AddNode("generate_response", func(ctx context.Context, state WorkflowState) (WorkflowState, error) {
		state.Steps = append(state.Steps, "generated")
		// Simulate AI generation (in real app, this would call an LLM)
		state.Message = fmt.Sprintf("Processed: %s (steps: %v)", state.Message, state.Steps)
		return state, nil
	})

	// Define edges
	workflow.SetEntryPoint("validate_input")
	workflow.AddEdge("validate_input", "process_message")
	workflow.AddEdge("process_message", "generate_response")
	workflow.AddEdge("generate_response", graph.END)

	return workflow
}

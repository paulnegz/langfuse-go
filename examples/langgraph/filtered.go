package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/paulnegz/langfuse-go/langgraph"
	"github.com/tmc/langgraphgo/graph"
)

// Example: Using filtered hooks for selective tracing
func main() {
	// Set up Langfuse credentials
	os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

	// Create base hook
	baseHook := langgraph.NewHook(
		langgraph.WithTraceName("filtered_workflow"),
		langgraph.WithMetadata(map[string]interface{}{
			"example": "filtering",
		}),
	)

	// Create filtered hook that only traces significant events
	filteredHook := langgraph.NewFilteredHook(
		baseHook,
		langgraph.EventFilter{
			// Only include node events, skip edge traversals
			IncludeEvents: []graph.TraceEvent{
				graph.TraceEventGraphStart,
				graph.TraceEventGraphEnd,
				graph.TraceEventNodeStart,
				graph.TraceEventNodeEnd,
				graph.TraceEventNodeError,
			},
			// Only trace operations longer than 50ms
			MinDuration: 50 * time.Millisecond,
		},
	)

	// Create workflow with many quick operations
	workflow := createNoisyWorkflow()

	// Set up tracing
	tracer := graph.NewTracer()
	tracer.AddHook(filteredHook)

	// Execute
	compiled := workflow.Compile()
	tracedWorkflow := graph.NewTracedRunnable(compiled, tracer)

	input := map[string]interface{}{
		"iterations": 10,
	}

	baseHook.SetInitialInput(input)

	ctx := context.Background()
	result, err := tracedWorkflow.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("Filtered workflow completed: %v\n", result)
	baseHook.Flush()
}

// createNoisyWorkflow creates a workflow with many quick operations
func createNoisyWorkflow() *graph.StateGraph {
	type NoisyState struct {
		Iterations    int
		QuickOps      int
		SlowOps       int
		Results       []string
	}

	workflow := graph.NewStateGraph(NoisyState{})

	// Quick operation (will be filtered out)
	workflow.AddNode("quick_op", func(ctx context.Context, state NoisyState) (NoisyState, error) {
		// Very fast operation - under threshold
		time.Sleep(10 * time.Millisecond)
		state.QuickOps++
		state.Results = append(state.Results, fmt.Sprintf("quick_%d", state.QuickOps))
		return state, nil
	})

	// Slow operation (will be traced)
	workflow.AddNode("slow_op", func(ctx context.Context, state NoisyState) (NoisyState, error) {
		// Slower operation - above threshold
		time.Sleep(100 * time.Millisecond)
		state.SlowOps++
		state.Results = append(state.Results, fmt.Sprintf("slow_%d", state.SlowOps))
		log.Printf("Slow operation %d completed", state.SlowOps)
		return state, nil
	})

	// AI simulation (will be traced)
	workflow.AddNode("ai_operation", func(ctx context.Context, state NoisyState) (NoisyState, error) {
		// Simulate AI call
		time.Sleep(200 * time.Millisecond)
		state.Results = append(state.Results, "ai_result")
		log.Println("AI operation completed")
		return state, nil
	})

	// Loop controller
	workflow.AddNode("controller", func(ctx context.Context, state NoisyState) (NoisyState, error) {
		state.Iterations--
		log.Printf("Iterations remaining: %d", state.Iterations)
		return state, nil
	})

	// Define flow
	workflow.SetEntryPoint("controller")
	
	workflow.AddConditionalEdges("controller",
		func(ctx context.Context, state NoisyState) string {
			if state.Iterations > 7 {
				return "quick_op"
			} else if state.Iterations > 3 {
				return "slow_op"
			} else if state.Iterations > 0 {
				return "ai_operation"
			}
			return "end"
		},
		map[string]string{
			"quick_op":     "quick_op",
			"slow_op":      "slow_op",
			"ai_operation": "ai_operation",
			"end":          graph.END,
		},
	)

	// Loop back from operations
	workflow.AddEdge("quick_op", "controller")
	workflow.AddEdge("slow_op", "controller")
	workflow.AddEdge("ai_operation", "controller")

	return workflow
}
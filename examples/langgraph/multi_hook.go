package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/paulnegz/langfuse-go/langgraph"
	"github.com/tmc/langgraphgo/graph"
)

// CustomLoggingHook implements a simple logging hook
type CustomLoggingHook struct {
	logFile *os.File
}

func NewCustomLoggingHook(filename string) (*CustomLoggingHook, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &CustomLoggingHook{logFile: file}, nil
}

func (h *CustomLoggingHook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	logEntry := fmt.Sprintf("[%s] Event: %s, Node: %s, Duration: %v\n",
		span.StartTime.Format("15:04:05"),
		span.Event,
		span.NodeName,
		span.Duration,
	)
	h.logFile.WriteString(logEntry)
}

func (h *CustomLoggingHook) Close() {
	h.logFile.Close()
}

// MetricsHook collects metrics
type MetricsHook struct {
	nodeCount   int
	totalTime   int64
	errorCount  int
}

func NewMetricsHook() *MetricsHook {
	return &MetricsHook{}
}

func (h *MetricsHook) OnEvent(ctx context.Context, span *graph.TraceSpan) {
	switch span.Event {
	case graph.TraceEventNodeEnd:
		h.nodeCount++
		h.totalTime += span.Duration.Milliseconds()
	case graph.TraceEventNodeError:
		h.errorCount++
	}
}

func (h *MetricsHook) PrintMetrics() {
	fmt.Printf("\n=== Metrics ===\n")
	fmt.Printf("Nodes executed: %d\n", h.nodeCount)
	fmt.Printf("Total time: %dms\n", h.totalTime)
	fmt.Printf("Errors: %d\n", h.errorCount)
	if h.nodeCount > 0 {
		fmt.Printf("Avg node time: %dms\n", h.totalTime/int64(h.nodeCount))
	}
}

// Example: Using multiple hooks together
func main() {
	// Set up Langfuse credentials
	os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

	// Create different hooks
	langfuseHook := langgraph.NewHook(
		langgraph.WithTraceName("multi_hook_demo"),
		langgraph.WithMetadata(map[string]interface{}{
			"hooks": "multiple",
			"demo":  true,
		}),
	)

	loggingHook, err := NewCustomLoggingHook("workflow.log")
	if err != nil {
		log.Fatalf("Failed to create logging hook: %v", err)
	}
	defer loggingHook.Close()

	metricsHook := NewMetricsHook()

	// Combine all hooks
	multiHook := langgraph.NewMultiHook(
		langfuseHook,
		loggingHook,
		metricsHook,
	)

	// Create workflow
	workflow := createDemoWorkflow()

	// Set up tracing with multi-hook
	tracer := graph.NewTracer()
	tracer.AddHook(multiHook)

	// Execute
	compiled := workflow.Compile()
	tracedWorkflow := graph.NewTracedRunnable(compiled, tracer)

	input := map[string]interface{}{
		"value": 42,
		"operation": "multiply",
	}

	langfuseHook.SetInitialInput(input)

	ctx := context.Background()
	result, err := tracedWorkflow.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nWorkflow result: %v\n", result)
	
	// Flush Langfuse
	langfuseHook.Flush()
	
	// Print metrics
	metricsHook.PrintMetrics()
	
	fmt.Println("\nCheck workflow.log for detailed logging!")
}

// createDemoWorkflow creates a demo workflow
func createDemoWorkflow() *graph.StateGraph {
	type DemoState struct {
		Value     int
		Operation string
		Result    int
		History   []string
	}

	workflow := graph.NewStateGraph(DemoState{})

	// Validation node
	workflow.AddNode("validate", func(ctx context.Context, state DemoState) (DemoState, error) {
		if state.Value <= 0 {
			return state, fmt.Errorf("value must be positive")
		}
		state.History = append(state.History, "validated")
		return state, nil
	})

	// Processing nodes
	workflow.AddNode("multiply", func(ctx context.Context, state DemoState) (DemoState, error) {
		state.Result = state.Value * 2
		state.History = append(state.History, fmt.Sprintf("multiplied: %d", state.Result))
		return state, nil
	})

	workflow.AddNode("square", func(ctx context.Context, state DemoState) (DemoState, error) {
		state.Result = state.Value * state.Value
		state.History = append(state.History, fmt.Sprintf("squared: %d", state.Result))
		return state, nil
	})

	workflow.AddNode("ai_enhance", func(ctx context.Context, state DemoState) (DemoState, error) {
		// Simulate AI enhancement
		state.Result = state.Result + 10
		state.History = append(state.History, fmt.Sprintf("ai_enhanced: %d", state.Result))
		return state, nil
	})

	// Final node
	workflow.AddNode("finalize", func(ctx context.Context, state DemoState) (DemoState, error) {
		state.History = append(state.History, fmt.Sprintf("final_result: %d", state.Result))
		log.Printf("Processing complete. History: %v", state.History)
		return state, nil
	})

	// Define flow
	workflow.SetEntryPoint("validate")
	workflow.AddEdge("validate", "multiply")
	
	workflow.AddConditionalEdges("multiply",
		func(ctx context.Context, state DemoState) string {
			if state.Operation == "square" {
				return "square"
			}
			return "ai_enhance"
		},
		map[string]string{
			"square":     "square",
			"ai_enhance": "ai_enhance",
		},
	)
	
	workflow.AddEdge("square", "ai_enhance")
	workflow.AddEdge("ai_enhance", "finalize")
	workflow.AddEdge("finalize", graph.END)

	return workflow
}
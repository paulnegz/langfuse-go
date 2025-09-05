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

// Example: Streaming workflow with real-time tracing
func main() {
	// Set up Langfuse credentials
	_ = os.Setenv("LANGFUSE_PUBLIC_KEY", "your_public_key")
	_ = os.Setenv("LANGFUSE_SECRET_KEY", "your_secret_key")

	// Create streaming workflow
	workflow := createStreamingWorkflow()

	// Create hook with streaming optimizations
	hook := langgraph.NewHook(
		langgraph.WithTraceName("streaming_workflow"),
		langgraph.WithSessionID("stream-session-001"),
		langgraph.WithMetadata(map[string]interface{}{
			"mode":       "streaming",
			"batch_size": 10,
		}),
		langgraph.WithAutoFlush(true), // Important for streaming
	)

	// Create traced runnable using helper
	tracedWorkflow := langgraph.NewTracedRunnable(
		workflow.Compile(),
		hook,
	)

	// Prepare input
	input := map[string]interface{}{
		"text":       "Process this text in chunks",
		"chunk_size": 5,
	}

	// Execute with streaming
	ctx := context.Background()
	outputChan, errorChan := tracedWorkflow.Stream(ctx, input)

	// Process streaming results
	fmt.Println("Starting streaming workflow...")

	done := false
	for !done {
		select {
		case output, ok := <-outputChan:
			if !ok {
				done = true
				break
			}
			fmt.Printf("Received chunk: %v\n", output)
		case err, ok := <-errorChan:
			if ok && err != nil {
				log.Printf("Stream error: %v", err)
			}
		case <-time.After(5 * time.Second):
			log.Println("Stream timeout")
			done = true
		}
	}

	fmt.Println("Streaming completed!")

	// Ensure final flush
	hook.Flush()
}

// createStreamingWorkflow creates a workflow that processes data in streams
func createStreamingWorkflow() *graph.StateGraph {
	type StreamState struct {
		Text       string
		ChunkSize  int
		Chunks     []string
		Processed  []string
		CurrentIdx int
	}

	workflow := graph.NewStateGraph(StreamState{})

	// Split text into chunks
	workflow.AddNode("chunker", func(ctx context.Context, state StreamState) (StreamState, error) {
		// Split text into chunks
		text := state.Text
		chunkSize := state.ChunkSize
		if chunkSize <= 0 {
			chunkSize = 5
		}

		state.Chunks = []string{}
		words := splitIntoWords(text)

		for i := 0; i < len(words); i += chunkSize {
			end := i + chunkSize
			if end > len(words) {
				end = len(words)
			}
			chunk := joinWords(words[i:end])
			state.Chunks = append(state.Chunks, chunk)
		}

		state.CurrentIdx = 0
		log.Printf("Split into %d chunks", len(state.Chunks))
		return state, nil
	})

	// Process each chunk (simulating streaming)
	workflow.AddNode("process_chunk", func(ctx context.Context, state StreamState) (StreamState, error) {
		if state.CurrentIdx >= len(state.Chunks) {
			return state, nil
		}

		chunk := state.Chunks[state.CurrentIdx]

		// Simulate processing delay
		time.Sleep(200 * time.Millisecond)

		// Process the chunk (in real app, this might be an LLM call)
		processed := fmt.Sprintf("[PROCESSED: %s]", chunk)
		state.Processed = append(state.Processed, processed)

		log.Printf("Processed chunk %d/%d", state.CurrentIdx+1, len(state.Chunks))
		state.CurrentIdx++

		return state, nil
	})

	// Aggregate results
	workflow.AddNode("aggregate", func(ctx context.Context, state StreamState) (StreamState, error) {
		// Combine all processed chunks
		result := joinWords(state.Processed)
		state.Text = result

		log.Printf("Aggregated %d processed chunks", len(state.Processed))
		return state, nil
	})

	// Define flow with loop for chunk processing
	workflow.SetEntryPoint("chunker")
	workflow.AddEdge("chunker", "process_chunk")

	// Conditional edge to loop or continue
	workflow.AddConditionalEdges("process_chunk",
		func(ctx context.Context, state StreamState) string {
			if state.CurrentIdx < len(state.Chunks) {
				return "process_chunk" // Loop back
			}
			return "aggregate" // Move to aggregation
		},
		map[string]string{
			"process_chunk": "process_chunk",
			"aggregate":     "aggregate",
		},
	)

	workflow.AddEdge("aggregate", graph.END)

	return workflow
}

// Helper functions
func splitIntoWords(text string) []string {
	// Simple word splitting (in production, use proper tokenization)
	words := []string{}
	current := ""

	for _, ch := range text {
		if ch == ' ' || ch == '\n' || ch == '\t' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		words = append(words, current)
	}

	return words
}

func joinWords(words []string) string {
	result := ""
	for i, word := range words {
		if i > 0 {
			result += " "
		}
		result += word
	}
	return result
}

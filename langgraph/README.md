# LangGraph Integration for Langfuse Go SDK

This package provides seamless integration between [LangGraph](https://github.com/paulnegz/langgraphgo) and [Langfuse](https://langfuse.com) for Go applications, enabling comprehensive tracing and observability of your graph-based workflows.

## Features

- 🔍 **Automatic Tracing**: Capture detailed traces of LangGraph workflow execution
- 🎯 **AI Operation Detection**: Automatically identify and track LLM calls with token usage
- 📊 **Hierarchical Spans**: Maintain parent-child relationships between operations
- 🏷️ **Metadata & Tags**: Add custom metadata, tags, and user/session IDs to traces
- ⚡ **Performance**: Async batching and efficient trace ingestion
- 🔧 **Flexible Configuration**: Multiple configuration options and builders

## Installation

```bash
go get github.com/paulnegz/langfuse-go
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/paulnegz/langfuse-go/langgraph"
    "github.com/paulnegz/langgraphgo/graph"
)

func main() {
    // Create your LangGraph workflow
    workflow := graph.NewGraph()
    // ... configure your workflow ...
    
    // Create Langfuse hook
    hook := langgraph.NewHook(
        langgraph.WithTraceName("my_workflow"),
        langgraph.WithSessionID("session-123"),
        langgraph.WithUserID("user-456"),
    )
    
    // Create tracer and add hook
    tracer := graph.NewTracer()
    tracer.AddHook(hook)
    
    // Create traced runnable
    tracedWorkflow := graph.NewTracedRunnable(workflow.Compile(), tracer)
    
    // Execute with tracing
    result, err := tracedWorkflow.Invoke(context.Background(), input)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Configuration

### Environment Variables

Set your Langfuse credentials:
```bash
export LANGFUSE_PUBLIC_KEY=your_public_key
export LANGFUSE_SECRET_KEY=your_secret_key
export LANGFUSE_HOST=https://cloud.langfuse.com  # optional
```

### Hook Configuration Options

```go
hook := langgraph.NewHook(
    // Enable/disable auto-flushing at graph end
    langgraph.WithAutoFlush(true),
    
    // Set custom trace name
    langgraph.WithTraceName("customer_support_bot"),
    
    // Add session tracking
    langgraph.WithSessionID("session-123"),
    
    // Add user tracking
    langgraph.WithUserID("user-456"),
    
    // Add custom metadata
    langgraph.WithMetadata(map[string]interface{}{
        "environment": "production",
        "version": "1.0.0",
    }),
    
    // Add tags for filtering
    langgraph.WithTags([]string{"production", "customer-support"}),
)
```

## Usage Patterns

### Basic Workflow Tracing

```go
// Simple workflow tracing
hook := langgraph.NewHook()
tracer := graph.NewTracer()
tracer.AddHook(hook)

// Set initial input for proper trace context
hook.SetInitialInput(initialInput)

tracedWorkflow := graph.NewTracedRunnable(workflow, tracer)
result, err := tracedWorkflow.Invoke(ctx, initialInput)
```

### Using the Builder Pattern

```go
hook := langgraph.NewBuilder().
    WithTraceName("complex_workflow").
    WithSessionID("session-789").
    WithUserID("user-123").
    WithMetadata(map[string]interface{}{
        "department": "engineering",
        "priority": "high",
    }).
    WithTags("production", "critical").
    WithAutoFlush(true).
    Build()
```

### Using TracedRunnable Helper

```go
// Simplified tracing with helper
tracedWorkflow := langgraph.NewTracedRunnable(
    workflow.Compile(),
    langgraph.NewHook(),
)

result, err := tracedWorkflow.Invoke(ctx, input)
```

### Event Filtering

```go
// Create filtered hook to reduce noise
filteredHook := langgraph.NewFilteredHook(
    langgraph.NewHook(),
    langgraph.EventFilter{
        // Only include specific events
        IncludeEvents: []graph.TraceEvent{
            graph.TraceEventNodeStart,
            graph.TraceEventNodeEnd,
        },
        // Filter out short-running operations
        MinDuration: 100 * time.Millisecond,
    },
)
```

### Multiple Hooks

```go
// Combine multiple trace destinations
multiHook := langgraph.NewMultiHook(
    langgraph.NewHook(),        // Langfuse
    customLoggingHook,           // Custom logging
    metricsHook,                 // Metrics collection
)

tracer := graph.NewTracer()
tracer.AddHook(multiHook)
```

### Manual Flushing

```go
hook := langgraph.NewHook(
    langgraph.WithAutoFlush(false), // Disable auto-flush
)

// ... execute workflow ...

// Manually flush traces
hook.Flush()
```

## AI Operation Detection

The integration automatically detects AI/LLM operations based on node naming patterns. Nodes containing these keywords are tracked as AI generations:

- `ai`, `llm`, `generate`, `completion`, `chat`
- `gpt`, `claude`, `gemini`, `openai`

For AI operations, the integration captures:
- Model name and parameters
- Token usage (input/output/total)
- Generation metadata

You can also explicitly mark nodes as AI operations by including model information in metadata:

```go
node.WithMetadata(map[string]interface{}{
    "model": "gpt-4",
    "temperature": 0.7,
    "max_tokens": 2048,
    "usage": map[string]interface{}{
        "input": 150,
        "output": 250,
    },
})
```

## Examples

### Customer Support Bot

```go
// See examples/langgraph/customer_support.go
hook := langgraph.NewHook(
    langgraph.WithTraceName("customer_support_bot"),
    langgraph.WithSessionID(sessionID),
    langgraph.WithUserID(userID),
    langgraph.WithMetadata(map[string]interface{}{
        "channel": "web_chat",
        "language": "en",
    }),
)
```

### Data Processing Pipeline

```go
// See examples/langgraph/data_pipeline.go
hook := langgraph.NewHook(
    langgraph.WithTraceName("etl_pipeline"),
    langgraph.WithTags([]string{"batch", "daily"}),
    langgraph.WithMetadata(map[string]interface{}{
        "source": "database",
        "destination": "warehouse",
    }),
)
```

### Multi-Agent System

```go
// See examples/langgraph/multi_agent.go
hook := langgraph.NewHook(
    langgraph.WithTraceName("multi_agent_collaboration"),
    langgraph.WithMetadata(map[string]interface{}{
        "agent_count": 3,
        "coordination": "hierarchical",
    }),
)
```

## Best Practices

1. **Always Set Initial Input**: Call `hook.SetInitialInput()` before execution to capture the complete context
2. **Use Meaningful Names**: Set descriptive trace names for easy identification in Langfuse UI
3. **Add Context**: Use session and user IDs to group related traces
4. **Tag Strategically**: Use tags for filtering traces by environment, feature, or priority
5. **Monitor Performance**: Use event filtering to reduce overhead in high-throughput scenarios
6. **Flush Appropriately**: Use auto-flush for real-time monitoring, manual flush for batch operations

## Troubleshooting

### Traces Not Appearing

1. Check environment variables are set correctly
2. Verify network connectivity to Langfuse
3. Enable debug logging: `export LANGFUSE_DEBUG=true`
4. Ensure `Flush()` is called if auto-flush is disabled

### Missing Metadata

- Call `SetInitialInput()` to capture workflow input
- Add metadata at hook creation or in node configuration
- Check that metadata values are serializable

### Performance Issues

- Use event filtering to reduce trace volume
- Enable batching with appropriate flush intervals
- Consider sampling for high-volume workflows

## API Reference

### Hook Creation

- `NewHook(opts ...Option) *Hook` - Create hook with options
- `NewHookWithClient(client *langfuse.Langfuse, opts ...Option) *Hook` - Create with existing client
- `NewBuilder() *TraceHookBuilder` - Create using builder pattern

### Configuration Options

- `WithAutoFlush(enabled bool)` - Enable/disable automatic flushing
- `WithMetadata(metadata map[string]interface{})` - Add default metadata
- `WithTraceName(name string)` - Set trace name
- `WithSessionID(id string)` - Set session ID
- `WithUserID(id string)` - Set user ID
- `WithTags(tags []string)` - Add trace tags

### Hook Methods

- `SetInitialInput(input interface{})` - Set workflow input
- `OnEvent(ctx context.Context, span *graph.TraceSpan)` - Handle trace events
- `Flush()` - Manually flush pending traces

### Helper Types

- `TracedRunnable` - Wrapper for traced execution
- `FilteredHook` - Event filtering wrapper
- `MultiHook` - Multiple hook aggregator

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This package is part of the langfuse-go SDK and follows the same license terms.
# LangGraph Integration Examples

This directory contains comprehensive examples demonstrating how to use the Langfuse-LangGraph integration in various scenarios.

## Prerequisites

Before running these examples, ensure you have:

1. **Set up Langfuse credentials:**
   ```bash
   export LANGFUSE_PUBLIC_KEY=your_public_key
   export LANGFUSE_SECRET_KEY=your_secret_key
   export LANGFUSE_HOST=https://cloud.langfuse.com  # optional
   ```

2. **Install dependencies:**
   ```bash
   go get github.com/henomis/langfuse-go
   go get github.com/paulnegz/langgraphgo
   ```

## Examples Overview

### 1. Basic Example (`basic.go`)
**Purpose:** Demonstrates fundamental integration setup and simple workflow tracing.

**Key Features:**
- Basic hook configuration
- Simple multi-step workflow
- Manual input setting for trace context
- Explicit flushing

**Use Case:** Getting started with Langfuse tracing in LangGraph workflows.

```bash
go run basic.go
```

### 2. Advanced Example (`advanced.go`)
**Purpose:** Shows complex workflow patterns with conditional routing and AI operations.

**Key Features:**
- Builder pattern for hook configuration
- Conditional routing based on state
- AI operation detection and tracking
- Cache integration
- Complex metadata and error handling

**Use Case:** Production-grade workflows with sophisticated logic and AI integration.

```bash
go run advanced.go
```

### 3. Streaming Example (`streaming.go`)
**Purpose:** Demonstrates real-time streaming workflows with continuous tracing.

**Key Features:**
- Streaming data processing
- Chunk-based operations
- Loop-back patterns
- Real-time trace updates
- Timeout handling

**Use Case:** Processing large datasets or real-time data streams with observability.

```bash
go run streaming.go
```

### 4. Filtered Tracing (`filtered.go`)
**Purpose:** Shows how to reduce trace noise by filtering events.

**Key Features:**
- Event type filtering
- Duration-based filtering
- Selective tracing of significant operations
- Performance optimization

**Use Case:** High-throughput systems where trace volume needs to be managed.

```bash
go run filtered.go
```

### 5. Multi-Hook Example (`multi_hook.go`)
**Purpose:** Demonstrates using multiple trace destinations simultaneously.

**Key Features:**
- Multiple hook implementations
- Custom logging hook
- Metrics collection hook
- Combined tracing strategies
- Hook composition

**Use Case:** Systems requiring multiple observability backends or custom trace processing.

```bash
go run multi_hook.go
```

## Example Patterns

### Basic Setup Pattern
```go
// 1. Create hook
hook := langgraph.NewHook(
    langgraph.WithTraceName("my_workflow"),
)

// 2. Create tracer
tracer := graph.NewTracer()
tracer.AddHook(hook)

// 3. Create traced runnable
traced := graph.NewTracedRunnable(workflow.Compile(), tracer)

// 4. Set initial input
hook.SetInitialInput(input)

// 5. Execute
result, err := traced.Invoke(ctx, input)
```

### Builder Pattern
```go
hook := langgraph.NewBuilder().
    WithTraceName("workflow").
    WithSessionID("session-123").
    WithUserID("user-456").
    WithMetadata(metadata).
    WithTags("production", "critical").
    Build()
```

### Filtering Pattern
```go
filtered := langgraph.NewFilteredHook(
    baseHook,
    langgraph.EventFilter{
        IncludeEvents: []graph.TraceEvent{
            graph.TraceEventNodeStart,
            graph.TraceEventNodeEnd,
        },
        MinDuration: 100 * time.Millisecond,
    },
)
```

### Multi-Hook Pattern
```go
multi := langgraph.NewMultiHook(
    langfuseHook,
    customHook,
    metricsHook,
)
```

## Common Scenarios

### 1. Development Environment
Use basic configuration with verbose tracing:
```go
hook := langgraph.NewHook(
    langgraph.WithTraceName("dev_workflow"),
    langgraph.WithMetadata(map[string]interface{}{
        "environment": "development",
        "debug": true,
    }),
)
```

### 2. Production Environment
Use filtered tracing with performance optimizations:
```go
hook := langgraph.NewFilteredHook(
    langgraph.NewHook(
        langgraph.WithTraceName("prod_workflow"),
        langgraph.WithAutoFlush(false),
    ),
    langgraph.EventFilter{
        MinDuration: 50 * time.Millisecond,
        ExcludeEvents: []graph.TraceEvent{
            graph.TraceEventEdgeTraversal,
        },
    },
)
```

### 3. AI-Powered Workflows
Track model usage and token counts:
```go
// In your AI node, add metadata
state.Metadata = map[string]interface{}{
    "model": "gpt-4",
    "temperature": 0.7,
    "max_tokens": 2048,
    "usage": map[string]interface{}{
        "input": inputTokens,
        "output": outputTokens,
    },
}
```

### 4. Multi-Tenant Systems
Track user and session context:
```go
hook := langgraph.NewHook(
    langgraph.WithUserID(userID),
    langgraph.WithSessionID(sessionID),
    langgraph.WithMetadata(map[string]interface{}{
        "tenant_id": tenantID,
        "subscription": "premium",
    }),
)
```

## Debugging Tips

1. **Enable Debug Logging:**
   ```bash
   export LANGFUSE_DEBUG=true
   ```

2. **Check Trace Delivery:**
   - Always call `hook.Flush()` when debugging
   - Check Langfuse UI for trace appearance
   - Verify network connectivity

3. **Common Issues:**
   - **Missing Traces:** Ensure credentials are set correctly
   - **Incomplete Traces:** Call `SetInitialInput()` before execution
   - **Performance:** Use filtering for high-volume workflows

## Performance Considerations

1. **Batching:** Traces are batched automatically for efficiency
2. **Filtering:** Use event filters to reduce trace volume
3. **Async Processing:** Traces are sent asynchronously by default
4. **Manual Flushing:** Control when traces are sent with `WithAutoFlush(false)`

## Best Practices

1. **Always set initial input** for complete trace context
2. **Use meaningful trace names** for easy identification
3. **Add relevant metadata** for debugging and filtering
4. **Implement error handling** in your nodes
5. **Use appropriate filtering** in production
6. **Monitor trace volume** to control costs

## Further Reading

- [Langfuse Documentation](https://langfuse.com/docs)
- [LangGraph Go Documentation](https://github.com/paulnegz/langgraphgo)
- [Integration Guide](../README.md)
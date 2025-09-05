# Langfuse Go SDK


[![GoDoc](https://godoc.org/github.com/henomis/langfuse-go?status.svg)](https://godoc.org/github.com/henomis/langfuse-go) [![Go Report Card](https://goreportcard.com/badge/github.com/henomis/langfuse-go)](https://goreportcard.com/report/github.com/henomis/langfuse-go) [![GitHub release](https://img.shields.io/github/release/henomis/langfuse-go.svg)](https://github.com/henomis/langfuse-go/releases)

This is [Langfuse](https://langfuse.com)'s **unofficial** Go client, designed to enable you to use Langfuse's services easily from your own applications.

## Langfuse

[Langfuse](https://langfuse.com) traces, evals, prompt management and metrics to debug and improve your LLM application.

## Features

- 🔍 **Comprehensive Tracing**: Track LLM calls, API requests, and custom operations
- 📊 **Performance Metrics**: Monitor latency, token usage, and costs
- 🎯 **Scoring & Evaluation**: Add quality scores and user feedback to traces
- 🔄 **Async Processing**: Efficient batching and background ingestion
- 🔗 **LangGraph Integration**: First-class support for LangGraph workflows
- 🛠️ **Simple API**: Clean, idiomatic Go interface
- 🔄 **LangChain Callbacks**: Compatible callback handlers in `/langchain` package


## API support

| **Index Operations**  | **Status** |
| --- | --- |
| Trace | 🟢 | 
| Generation | 🟢 |
| Span | 🟢 |
| Event | 🟢 |
| Score | 🟢 |




## Getting started

### Installation

You can load langfuse-go into your project by using:
```
go get github.com/paulnegz/langfuse-go
```


### Configuration
Just like the official Python SDK, these three environment variables will be used to configure the Langfuse client:

- `LANGFUSE_HOST`: The host of the Langfuse service.
- `LANGFUSE_PUBLIC_KEY`: Your public key for the Langfuse service.
- `LANGFUSE_SECRET_KEY`: Your secret key for the Langfuse service.


### Usage

Please refer to the [examples folder](examples/cmd/) to see how to use the SDK.

#### LangGraph Integration

The SDK includes first-class support for tracing [LangGraph](https://github.com/paulnegz/langgraphgo) workflows:

```go
import (
    "github.com/paulnegz/langfuse-go/langgraph"
    "github.com/paulnegz/langgraphgo/graph"
)

// Create Langfuse hook for LangGraph
hook := langgraph.NewHook(
    langgraph.WithTraceName("my_workflow"),
    langgraph.WithSessionID("session-123"),
    langgraph.WithUserID("user-456"),
)

// Add to your graph tracer
tracer := graph.NewTracer()
tracer.AddHook(hook)

// Create traced workflow
tracedWorkflow := graph.NewTracedRunnable(workflow.Compile(), tracer)

// Execute with automatic tracing
result, err := tracedWorkflow.Invoke(ctx, input)
```

See the [LangGraph integration guide](langgraph/README.md) for detailed documentation and [examples](examples/langgraph/).

#### Basic SDK Usage

Here below a simple usage example:

```go
package main

import (
	"context"

	"github.com/paulnegz/langfuse-go"
	"github.com/paulnegz/langfuse-go/model"
)

func main() {
	l := langfuse.New()

	err := l.Trace(&model.Trace{Name: "test-trace"})
	if err != nil {
		panic(err)
	}

	err = l.Span(&model.Span{Name: "test-span"})
	if err != nil {
		panic(err)
	}

	err = l.Generation(
		&model.Generation{
			Name:  "test-generation",
			Model: "gpt-3.5-turbo",
			ModelParameters: model.M{
				"maxTokens":   "1000",
				"temperature": "0.9",
			},
			Input: []model.M{
				{
					"role":    "system",
					"content": "You are a helpful assistant.",
				},
				{
					"role":    "user",
					"content": "Please generate a summary of the following documents \nThe engineering department defined the following OKR goals...\nThe marketing department defined the following OKR goals...",
				},
			},
			Metadata: model.M{
				"key": "value",
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = l.Event(
		&model.Event{
			Name: "test-event",
			Metadata: model.M{
				"key": "value",
			},
			Input: model.M{
				"key": "value",
			},
			Output: model.M{
				"key": "value",
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = l.GenerationEnd(
		&model.Generation{
			Output: model.M{
				"completion": "The Q3 OKRs contain goals for multiple teams...",
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = l.Score(
		&model.Score{
			Name:  "test-score",
			Value: 0.9,
		},
	)
	if err != nil {
		panic(err)
	}

	err = l.SpanEnd(&model.Span{})
	if err != nil {
		panic(err)
	}

	l.Flush(context.Background())

}
```

## Who uses langfuse-go?

* [LangGraphGo](https://github.com/paulnegz/langgraphgo) Go implementation of LangGraph for building stateful, multi-actor LLM applications

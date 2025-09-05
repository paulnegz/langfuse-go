package main

import (
	"context"
	"log"

	langfuse "github.com/paulnegz/langfuse-go"
	"github.com/paulnegz/langfuse-go/model"
)

func main() {
	l := langfuse.New(context.Background())

	trace, err := l.Trace(&model.Trace{Name: "test-trace"})
	if err != nil {
		panic(err)
	}

	span, spanErr := l.Span(&model.Span{Name: "test-span", TraceID: trace.ID}, nil)
	if spanErr != nil {
		panic(spanErr)
	}

	generation, genErr := l.Generation(
		&model.Generation{
			TraceID: trace.ID,
			Name:    "test-generation",
			Model:   "gpt-3.5-turbo",
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
		&span.ID,
	)
	if genErr != nil {
		panic(genErr)
	}

	_, eventErr := l.Event(
		&model.Event{
			Name:    "test-event",
			TraceID: trace.ID,
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
		&generation.ID,
	)
	if eventErr != nil {
		panic(eventErr)
	}

	generation.Output = model.M{
		"completion": "The Q3 OKRs contain goals for multiple teams...",
	}
	_, genEndErr := l.GenerationEnd(generation)
	if genEndErr != nil {
		panic(genEndErr)
	}

	_, scoreErr := l.Score(
		&model.Score{
			TraceID: trace.ID,
			Name:    "test-score",
			Value:   0.9,
		},
	)
	if scoreErr != nil {
		panic(scoreErr)
	}

	_, spanEndErr := l.SpanEnd(span)
	if spanEndErr != nil {
		panic(spanEndErr)
	}

	if err := l.Flush(context.Background()); err != nil {
		log.Printf("Failed to flush: %v", err)
	}

}

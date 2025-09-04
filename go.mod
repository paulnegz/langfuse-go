module github.com/paulnegz/langfuse-go

go 1.22.0

toolchain go1.24.2

require (
	github.com/google/uuid v1.6.0
	github.com/tmc/langgraphgo v0.8.0
)

replace github.com/tmc/langgraphgo => github.com/paulnegz/langgraphgo v0.8.0

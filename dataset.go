package langfuse

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/paulnegz/langfuse-go/model"
)

// Dataset represents a Langfuse dataset
type Dataset struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Items       []*DatasetItem         `json:"items"`
	client      *Langfuse
}

// DatasetItem represents an item in a dataset
type DatasetItem struct {
	ID             string                 `json:"id"`
	DatasetID      string                 `json:"datasetId"`
	Input          interface{}            `json:"input"`
	ExpectedOutput interface{}            `json:"expectedOutput,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	SourceTraceID  string                 `json:"sourceTraceId,omitempty"`
	SourceSpanID   string                 `json:"sourceSpanId,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
	client         *Langfuse
}

// DatasetRun represents an execution run of a dataset item
type DatasetRun struct {
	ID          string                 `json:"id"`
	DatasetID   string                 `json:"datasetId"`
	ItemID      string                 `json:"itemId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
	TraceID     string                 `json:"traceId"`
	SpanID      string                 `json:"spanId,omitempty"`
	StartedAt   time.Time              `json:"startedAt"`
	EndedAt     *time.Time             `json:"endedAt,omitempty"`
	client      *Langfuse
	item        *DatasetItem
}

// DatasetClient provides dataset management functionality
type DatasetClient struct {
	client *Langfuse
}

// NewDatasetClient creates a new dataset client
func (l *Langfuse) NewDatasetClient() *DatasetClient {
	return &DatasetClient{
		client: l,
	}
}

// GetDataset retrieves a dataset by name or ID
func (dc *DatasetClient) GetDataset(ctx context.Context, nameOrID string) (*Dataset, error) {
	// In real implementation, this would call the Langfuse API
	// For now, return a mock dataset
	dataset := &Dataset{
		ID:          uuid.New().String(),
		Name:        nameOrID,
		Description: "Dataset for " + nameOrID,
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Items:       make([]*DatasetItem, 0),
		client:      dc.client,
	}

	// Load items
	err := dataset.LoadItems(ctx)
	if err != nil {
		return nil, err
	}

	return dataset, nil
}

// CreateDataset creates a new dataset
func (dc *DatasetClient) CreateDataset(ctx context.Context, name string, description string, metadata map[string]interface{}) (*Dataset, error) {
	if name == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	dataset := &Dataset{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Items:       make([]*DatasetItem, 0),
		client:      dc.client,
	}

	// In real implementation, this would save to Langfuse API

	return dataset, nil
}

// ListDatasets retrieves all datasets with pagination
func (dc *DatasetClient) ListDatasets(ctx context.Context, page int, limit int) ([]*Dataset, error) {
	// In real implementation, this would call the Langfuse API with pagination
	datasets := make([]*Dataset, 0)
	return datasets, nil
}

// Dataset methods

// LoadItems loads all items for a dataset
func (d *Dataset) LoadItems(ctx context.Context) error {
	// In real implementation, this would fetch from Langfuse API
	// For now, create mock items
	d.Items = []*DatasetItem{
		{
			ID:             uuid.New().String(),
			DatasetID:      d.ID,
			Input:          map[string]interface{}{"query": "What is the capital of France?"},
			ExpectedOutput: map[string]interface{}{"answer": "Paris"},
			Metadata:       map[string]interface{}{"type": "qa"},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			client:         d.client,
		},
	}

	return nil
}

// CreateItem adds a new item to the dataset
func (d *Dataset) CreateItem(input interface{}, expectedOutput interface{}, metadata map[string]interface{}) (*DatasetItem, error) {
	item := &DatasetItem{
		ID:             uuid.New().String(),
		DatasetID:      d.ID,
		Input:          input,
		ExpectedOutput: expectedOutput,
		Metadata:       metadata,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		client:         d.client,
	}

	// In real implementation, save to API
	d.Items = append(d.Items, item)

	return item, nil
}

// CreateItemFromTrace creates a dataset item from an existing trace
func (d *Dataset) CreateItemFromTrace(traceID string, spanID string, metadata map[string]interface{}) (*DatasetItem, error) {
	// In real implementation, fetch trace/span data from API

	item := &DatasetItem{
		ID:            uuid.New().String(),
		DatasetID:     d.ID,
		SourceTraceID: traceID,
		SourceSpanID:  spanID,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		client:        d.client,
	}

	d.Items = append(d.Items, item)

	return item, nil
}

// GetItem retrieves a specific item by ID
func (d *Dataset) GetItem(itemID string) (*DatasetItem, error) {
	for _, item := range d.Items {
		if item.ID == itemID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("item not found: %s", itemID)
}

// DatasetItem methods

// Run creates a new run for this dataset item
func (di *DatasetItem) Run(name string, description string) (*DatasetRun, error) {
	run := &DatasetRun{
		ID:          uuid.New().String(),
		DatasetID:   di.DatasetID,
		ItemID:      di.ID,
		Name:        name,
		Description: description,
		Metadata:    make(map[string]interface{}),
		StartedAt:   time.Now(),
		client:      di.client,
		item:        di,
	}

	// Create associated trace
	startTime := time.Now()
	trace := &model.Trace{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("dataset-run-%s", name),
		Timestamp: &startTime,
		Input:     di.Input,
		Metadata: map[string]interface{}{
			"dataset_id":      di.DatasetID,
			"dataset_item_id": di.ID,
			"run_name":        name,
			"run_description": description,
		},
	}

	createdTrace, err := di.client.Trace(trace)
	if err != nil {
		return nil, err
	}

	run.TraceID = createdTrace.ID

	return run, nil
}

// RunWithContext creates a run with a custom context
type RunContext struct {
	run       *DatasetRun
	span      *model.Span
	startTime time.Time
}

// Start begins execution tracking for a run
func (dr *DatasetRun) Start() *RunContext {
	startTime := time.Now()

	// Create span for this run
	span := &model.Span{
		ID:        uuid.New().String(),
		TraceID:   dr.TraceID,
		Name:      dr.Name,
		StartTime: &startTime,
		Input:     dr.item.Input,
		Metadata:  dr.Metadata,
	}

	_, err := dr.client.Span(span, nil)
	if err != nil {
		log.Printf("Failed to create span: %v", err)
	}
	dr.SpanID = span.ID

	return &RunContext{
		run:       dr,
		span:      span,
		startTime: startTime,
	}
}

// End completes the run execution
func (rc *RunContext) End(output interface{}, err error) error {
	endTime := time.Now()
	rc.run.EndedAt = &endTime

	// Update span with results
	metadata := map[string]interface{}{
		"duration_ms": endTime.Sub(rc.startTime).Milliseconds(),
	}
	if err != nil {
		metadata["error"] = err.Error()
	}

	rc.span.EndTime = &endTime
	rc.span.Output = output
	rc.span.Metadata = metadata

	_, spanErr := rc.run.client.SpanEnd(rc.span)
	return spanErr
}

// Score adds a score to the run
func (rc *RunContext) Score(name string, value float64, comment string) error {
	score := &model.Score{
		ID:            uuid.New().String(),
		TraceID:       rc.run.TraceID,
		Name:          name,
		Value:         value,
		Comment:       comment,
		ObservationID: rc.run.SpanID,
	}

	_, err := rc.run.client.Score(score)
	return err
}

// DatasetEvaluator provides evaluation capabilities for datasets
type DatasetEvaluator struct {
	dataset   *Dataset
	evaluator func(input interface{}, expectedOutput interface{}, actualOutput interface{}) (float64, error)
}

// NewDatasetEvaluator creates a new dataset evaluator
func NewDatasetEvaluator(dataset *Dataset, evaluator func(interface{}, interface{}, interface{}) (float64, error)) *DatasetEvaluator {
	return &DatasetEvaluator{
		dataset:   dataset,
		evaluator: evaluator,
	}
}

// Evaluate runs evaluation on all dataset items
func (de *DatasetEvaluator) Evaluate(ctx context.Context, runner func(interface{}) (interface{}, error)) (*EvaluationResult, error) {
	results := &EvaluationResult{
		DatasetID:   de.dataset.ID,
		DatasetName: de.dataset.Name,
		StartedAt:   time.Now(),
		Items:       make([]*ItemResult, 0),
		Scores:      make(map[string]float64),
	}

	totalScore := 0.0

	for _, item := range de.dataset.Items {
		// Create run for this item
		run, err := item.Run("evaluation", "Automated evaluation run")
		if err != nil {
			continue
		}

		runCtx := run.Start()

		// Execute runner
		output, runErr := runner(item.Input)

		// Calculate score
		score := 0.0
		if runErr == nil && de.evaluator != nil {
			evalScore, evalErr := de.evaluator(item.Input, item.ExpectedOutput, output)
			if evalErr != nil {
				log.Printf("Evaluator error: %v", evalErr)
			} else {
				score = evalScore
			}
		}

		// End run and record score
		if endErr := runCtx.End(output, runErr); endErr != nil {
			log.Printf("Failed to end run context: %v", endErr)
		}
		if scoreErr := runCtx.Score("evaluation", score, ""); scoreErr != nil {
			log.Printf("Failed to record score: %v", scoreErr)
		}

		// Record result
		itemResult := &ItemResult{
			ItemID:         item.ID,
			Input:          item.Input,
			ExpectedOutput: item.ExpectedOutput,
			ActualOutput:   output,
			Score:          score,
			Error:          runErr,
			TraceID:        run.TraceID,
		}

		results.Items = append(results.Items, itemResult)
		totalScore += score
	}

	results.EndedAt = time.Now()

	// Calculate aggregate scores
	if len(results.Items) > 0 {
		results.Scores["average"] = totalScore / float64(len(results.Items))
	}

	return results, nil
}

// EvaluationResult contains the results of a dataset evaluation
type EvaluationResult struct {
	DatasetID   string                 `json:"datasetId"`
	DatasetName string                 `json:"datasetName"`
	StartedAt   time.Time              `json:"startedAt"`
	EndedAt     time.Time              `json:"endedAt"`
	Items       []*ItemResult          `json:"items"`
	Scores      map[string]float64     `json:"scores"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ItemResult contains the result of evaluating a single dataset item
type ItemResult struct {
	ItemID         string      `json:"itemId"`
	Input          interface{} `json:"input"`
	ExpectedOutput interface{} `json:"expectedOutput"`
	ActualOutput   interface{} `json:"actualOutput"`
	Score          float64     `json:"score"`
	Error          error       `json:"error,omitempty"`
	TraceID        string      `json:"traceId"`
}

// Convenience methods on Langfuse client

// GetDataset retrieves a dataset (convenience method)
func (l *Langfuse) GetDataset(ctx context.Context, nameOrID string) (*Dataset, error) {
	dc := l.NewDatasetClient()
	return dc.GetDataset(ctx, nameOrID)
}

// CreateDataset creates a new dataset (convenience method)
func (l *Langfuse) CreateDataset(ctx context.Context, name string, description string) (*Dataset, error) {
	dc := l.NewDatasetClient()
	return dc.CreateDataset(ctx, name, description, nil)
}

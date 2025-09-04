package langfuse

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MediaContent represents a media file or data
type MediaContent struct {
	ID          string    `json:"id"`
	Data        []byte    `json:"-"`              // Raw data (not serialized)
	DataURI     string    `json:"data,omitempty"` // Base64 data URI
	ContentType string    `json:"content_type"`
	FileName    string    `json:"file_name,omitempty"`
	Size        int       `json:"size"`
	Hash        string    `json:"hash"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty"`
	ReferenceID string    `json:"reference_id,omitempty"`
}

// NewMediaFromFile creates media content from a file path
func NewMediaFromFile(filePath string) (*MediaContent, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Detect content type
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	
	// Calculate hash
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	
	// Create data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
	
	return &MediaContent{
		ID:          uuid.New().String(),
		Data:        data,
		DataURI:     dataURI,
		ContentType: contentType,
		FileName:    filepath.Base(filePath),
		Size:        len(data),
		Hash:        hash,
	}, nil
}

// NewMediaFromBytes creates media content from raw bytes
func NewMediaFromBytes(data []byte, contentType string, fileName string) *MediaContent {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
	
	return &MediaContent{
		ID:          uuid.New().String(),
		Data:        data,
		DataURI:     dataURI,
		ContentType: contentType,
		FileName:    fileName,
		Size:        len(data),
		Hash:        hash,
	}
}

// NewMediaFromDataURI creates media content from a data URI
func NewMediaFromDataURI(dataURI string) (*MediaContent, error) {
	// Parse data URI format: data:[<mediatype>][;base64],<data>
	if !strings.HasPrefix(dataURI, "data:") {
		return nil, fmt.Errorf("invalid data URI format")
	}
	
	parts := strings.SplitN(dataURI[5:], ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data URI format")
	}
	
	// Parse media type and encoding
	mimeInfo := parts[0]
	encodedData := parts[1]
	
	contentType := "text/plain"
	isBase64 := false
	
	if mimeInfo != "" {
		mimeParts := strings.Split(mimeInfo, ";")
		contentType = mimeParts[0]
		for _, part := range mimeParts[1:] {
			if part == "base64" {
				isBase64 = true
			}
		}
	}
	
	// Decode data
	var data []byte
	var err error
	if isBase64 {
		data, err = base64.StdEncoding.DecodeString(encodedData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
	} else {
		data = []byte(encodedData)
	}
	
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	
	return &MediaContent{
		ID:          uuid.New().String(),
		Data:        data,
		DataURI:     dataURI,
		ContentType: contentType,
		Size:        len(data),
		Hash:        hash,
	}, nil
}

// ToReferenceString returns a reference string for this media
func (m *MediaContent) ToReferenceString() string {
	if m.ReferenceID != "" {
		return fmt.Sprintf("@media/%s", m.ReferenceID)
	}
	return fmt.Sprintf("@media/%s", m.ID)
}

// MediaUploader handles asynchronous media uploads
type MediaUploader struct {
	client      *Langfuse
	queue       chan *MediaUploadTask
	workers     int
	wg          sync.WaitGroup
	mu          sync.RWMutex
	uploads     map[string]*MediaUploadStatus
	dedupCache  map[string]string // hash -> reference_id
}

// MediaUploadTask represents a media upload task
type MediaUploadTask struct {
	Media    *MediaContent
	TraceID  string
	SpanID   string
	Callback func(referenceID string, err error)
}

// MediaUploadStatus tracks upload progress
type MediaUploadStatus struct {
	ID          string
	Status      string // "queued", "uploading", "completed", "failed"
	ReferenceID string
	Error       error
	StartedAt   time.Time
	CompletedAt *time.Time
}

// NewMediaUploader creates a new media uploader
func NewMediaUploader(client *Langfuse, workers int) *MediaUploader {
	if workers <= 0 {
		workers = 2
	}
	
	uploader := &MediaUploader{
		client:     client,
		queue:      make(chan *MediaUploadTask, 100),
		workers:    workers,
		uploads:    make(map[string]*MediaUploadStatus),
		dedupCache: make(map[string]string),
	}
	
	// Start workers
	for i := 0; i < workers; i++ {
		uploader.wg.Add(1)
		go uploader.worker()
	}
	
	return uploader
}

// Upload queues a media upload task
func (mu *MediaUploader) Upload(media *MediaContent, traceID string, spanID string) (string, error) {
	// Check dedup cache
	mu.mu.RLock()
	if refID, exists := mu.dedupCache[media.Hash]; exists {
		mu.mu.RUnlock()
		media.ReferenceID = refID
		return refID, nil
	}
	mu.mu.RUnlock()
	
	// Create upload status
	status := &MediaUploadStatus{
		ID:        media.ID,
		Status:    "queued",
		StartedAt: time.Now(),
	}
	
	mu.mu.Lock()
	mu.uploads[media.ID] = status
	mu.mu.Unlock()
	
	// Queue upload task
	task := &MediaUploadTask{
		Media:   media,
		TraceID: traceID,
		SpanID:  spanID,
	}
	
	select {
	case mu.queue <- task:
		return media.ID, nil
	default:
		return "", fmt.Errorf("upload queue is full")
	}
}

// UploadWithCallback queues a media upload with a callback
func (mu *MediaUploader) UploadWithCallback(media *MediaContent, traceID string, spanID string, callback func(string, error)) {
	task := &MediaUploadTask{
		Media:    media,
		TraceID:  traceID,
		SpanID:   spanID,
		Callback: callback,
	}
	
	mu.queue <- task
}

// worker processes upload tasks
func (mu *MediaUploader) worker() {
	defer mu.wg.Done()
	
	for task := range mu.queue {
		mu.processUpload(task)
	}
}

// processUpload handles a single upload task
func (mu *MediaUploader) processUpload(task *MediaUploadTask) {
	// Update status
	mu.mu.Lock()
	if status, exists := mu.uploads[task.Media.ID]; exists {
		status.Status = "uploading"
	}
	mu.mu.Unlock()
	
	// Simulate upload to Langfuse API
	// In real implementation, this would POST to the media endpoint
	time.Sleep(100 * time.Millisecond)
	
	// Generate reference ID (in real implementation, this comes from API)
	referenceID := fmt.Sprintf("media_%s", uuid.New().String())
	
	// Update dedup cache
	mu.mu.Lock()
	mu.dedupCache[task.Media.Hash] = referenceID
	if status, exists := mu.uploads[task.Media.ID]; exists {
		status.Status = "completed"
		status.ReferenceID = referenceID
		now := time.Now()
		status.CompletedAt = &now
	}
	mu.mu.Unlock()
	
	// Set reference ID on media
	task.Media.ReferenceID = referenceID
	now := time.Now()
	task.Media.UploadedAt = &now
	
	// Call callback if provided
	if task.Callback != nil {
		task.Callback(referenceID, nil)
	}
}

// GetStatus returns the upload status for a media ID
func (mu *MediaUploader) GetStatus(mediaID string) *MediaUploadStatus {
	mu.mu.RLock()
	defer mu.mu.RUnlock()
	return mu.uploads[mediaID]
}

// WaitForUpload waits for a specific upload to complete
func (mu *MediaUploader) WaitForUpload(mediaID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	
	for {
		status := mu.GetStatus(mediaID)
		if status == nil {
			return "", fmt.Errorf("upload not found: %s", mediaID)
		}
		
		if status.Status == "completed" {
			return status.ReferenceID, nil
		}
		
		if status.Status == "failed" {
			return "", status.Error
		}
		
		if time.Now().After(deadline) {
			return "", fmt.Errorf("upload timeout")
		}
		
		time.Sleep(100 * time.Millisecond)
	}
}

// Shutdown gracefully shuts down the uploader
func (mu *MediaUploader) Shutdown() {
	close(mu.queue)
	mu.wg.Wait()
}

// MediaProcessor provides utilities for processing media in traces
type MediaProcessor struct {
	uploader *MediaUploader
}

// NewMediaProcessor creates a new media processor
func NewMediaProcessor(uploader *MediaUploader) *MediaProcessor {
	return &MediaProcessor{
		uploader: uploader,
	}
}

// ProcessInput processes media in input data
func (mp *MediaProcessor) ProcessInput(input interface{}, traceID string) interface{} {
	return mp.processValue(input, traceID, "")
}

// ProcessOutput processes media in output data
func (mp *MediaProcessor) ProcessOutput(output interface{}, traceID string, spanID string) interface{} {
	return mp.processValue(output, traceID, spanID)
}

// processValue recursively processes media in values
func (mp *MediaProcessor) processValue(value interface{}, traceID string, spanID string) interface{} {
	switch v := value.(type) {
	case *MediaContent:
		// Upload media and return reference
		refID, err := mp.uploader.Upload(v, traceID, spanID)
		if err != nil {
			return v.ToReferenceString()
		}
		return fmt.Sprintf("@media/%s", refID)
		
	case map[string]interface{}:
		// Process map values
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = mp.processValue(val, traceID, spanID)
		}
		return result
		
	case []interface{}:
		// Process array values
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = mp.processValue(val, traceID, spanID)
		}
		return result
		
	default:
		return value
	}
}

// Global media uploader instance (optional, for convenience)
var globalUploader *MediaUploader
var globalUploaderOnce sync.Once

// GetGlobalUploader returns the global media uploader instance
func GetGlobalUploader(client *Langfuse) *MediaUploader {
	globalUploaderOnce.Do(func() {
		globalUploader = NewMediaUploader(client, 4)
	})
	return globalUploader
}

// Helper functions for media handling

// IsMediaReference checks if a string is a media reference
func IsMediaReference(s string) bool {
	return strings.HasPrefix(s, "@media/")
}

// ParseMediaReference extracts the reference ID from a media reference string
func ParseMediaReference(s string) (string, bool) {
	if !IsMediaReference(s) {
		return "", false
	}
	return strings.TrimPrefix(s, "@media/"), true
}

// MediaHelper provides convenience methods for media handling
type MediaHelper struct {
	client   *Langfuse
	uploader *MediaUploader
}

// NewMediaHelper creates a new media helper
func NewMediaHelper(client *Langfuse) *MediaHelper {
	return &MediaHelper{
		client:   client,
		uploader: GetGlobalUploader(client),
	}
}

// AttachImage attaches an image to a trace or span
func (mh *MediaHelper) AttachImage(filePath string, traceID string, spanID string) (string, error) {
	media, err := NewMediaFromFile(filePath)
	if err != nil {
		return "", err
	}
	
	return mh.uploader.Upload(media, traceID, spanID)
}

// AttachData attaches raw data as media
func (mh *MediaHelper) AttachData(data []byte, contentType string, name string, traceID string, spanID string) (string, error) {
	media := NewMediaFromBytes(data, contentType, name)
	return mh.uploader.Upload(media, traceID, spanID)
}
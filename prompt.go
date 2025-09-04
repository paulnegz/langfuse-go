package langfuse

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PromptType represents the type of prompt
type PromptType string

const (
	PromptTypeText PromptType = "text"
	PromptTypeChat PromptType = "chat"
)

// Prompt represents a versioned prompt template
type Prompt struct {
	Name    string                 `json:"name"`
	Version int                    `json:"version"`
	Type    PromptType             `json:"type"`
	Prompt  interface{}            `json:"prompt"` // string for text, []ChatMessage for chat
	Config  map[string]interface{} `json:"config"`
	Labels  []string               `json:"labels"`
}

// ChatMessage represents a chat message in a prompt template
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompiledPrompt represents a prompt with variables replaced
type CompiledPrompt struct {
	Type   PromptType
	Text   string        // For text prompts
	Chat   []ChatMessage // For chat prompts
	Config map[string]interface{}
}

// PromptClient provides prompt management functionality
type PromptClient struct {
	langfuse *Langfuse
	cache    *PromptCache
}

// NewPromptClient creates a new prompt client
func (l *Langfuse) NewPromptClient() *PromptClient {
	return &PromptClient{
		langfuse: l,
		cache:    NewPromptCache(60 * time.Second), // 60s TTL like Python
	}
}

// GetPrompt retrieves a prompt by name and optional version or label
func (pc *PromptClient) GetPrompt(ctx context.Context, name string, opts ...PromptOption) (*Prompt, error) {
	options := &promptOptions{
		version: -1, // Latest version by default
	}
	
	for _, opt := range opts {
		opt(options)
	}
	
	// Check cache first
	cacheKey := pc.buildCacheKey(name, options)
	if cached := pc.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}
	
	// Fetch from API
	prompt, err := pc.fetchPrompt(ctx, name, options)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	pc.cache.Set(cacheKey, prompt)
	
	return prompt, nil
}

// CreatePrompt creates a new prompt or version
func (pc *PromptClient) CreatePrompt(ctx context.Context, prompt *Prompt) (*Prompt, error) {
	// Validate prompt
	if prompt.Name == "" {
		return nil, fmt.Errorf("prompt name is required")
	}
	
	if prompt.Type != PromptTypeText && prompt.Type != PromptTypeChat {
		return nil, fmt.Errorf("invalid prompt type: %s", prompt.Type)
	}
	
	// Send to API (simplified for example)
	// In real implementation, this would call the Langfuse API
	
	// Invalidate cache for this prompt name
	pc.cache.InvalidatePrefix(prompt.Name)
	
	return prompt, nil
}

// Compile replaces variables in the prompt template
func (p *Prompt) Compile(variables map[string]interface{}) (*CompiledPrompt, error) {
	compiled := &CompiledPrompt{
		Type:   p.Type,
		Config: p.Config,
	}
	
	switch p.Type {
	case PromptTypeText:
		text, ok := p.Prompt.(string)
		if !ok {
			return nil, fmt.Errorf("invalid text prompt format")
		}
		compiled.Text = replaceVariables(text, variables)
		
	case PromptTypeChat:
		messages, ok := p.Prompt.([]ChatMessage)
		if !ok {
			// Try to convert from []interface{}
			if msgs, ok := p.Prompt.([]interface{}); ok {
				messages = make([]ChatMessage, 0, len(msgs))
				for _, msg := range msgs {
					if m, ok := msg.(map[string]interface{}); ok {
						messages = append(messages, ChatMessage{
							Role:    getString(m, "role"),
							Content: getString(m, "content"),
						})
					}
				}
			} else {
				return nil, fmt.Errorf("invalid chat prompt format")
			}
		}
		
		compiled.Chat = make([]ChatMessage, len(messages))
		for i, msg := range messages {
			compiled.Chat[i] = ChatMessage{
				Role:    msg.Role,
				Content: replaceVariables(msg.Content, variables),
			}
		}
		
	default:
		return nil, fmt.Errorf("unsupported prompt type: %s", p.Type)
	}
	
	return compiled, nil
}

// replaceVariables replaces {{variable}} placeholders with values
func replaceVariables(template string, variables map[string]interface{}) string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	
	return re.ReplaceAllStringFunc(template, func(match string) string {
		varName := re.FindStringSubmatch(match)[1]
		if value, ok := variables[varName]; ok {
			return fmt.Sprintf("%v", value)
		}
		return match // Keep original if variable not found
	})
}

// PromptOption configures prompt retrieval
type PromptOption func(*promptOptions)

type promptOptions struct {
	version int
	label   string
}

// WithVersion specifies a specific prompt version
func WithVersion(version int) PromptOption {
	return func(o *promptOptions) {
		o.version = version
	}
}

// WithLabel specifies a prompt label (e.g., "production", "latest")
func WithLabel(label string) PromptOption {
	return func(o *promptOptions) {
		o.label = label
	}
}

// fetchPrompt fetches a prompt from the API (simplified)
func (pc *PromptClient) fetchPrompt(ctx context.Context, name string, opts *promptOptions) (*Prompt, error) {
	// In real implementation, this would call the Langfuse API
	// For now, return a mock prompt
	return &Prompt{
		Name:    name,
		Version: 1,
		Type:    PromptTypeText,
		Prompt:  "Hello {{name}}, welcome to {{place}}!",
		Config:  map[string]interface{}{"temperature": 0.7},
		Labels:  []string{"production"},
	}, nil
}

func (pc *PromptClient) buildCacheKey(name string, opts *promptOptions) string {
	if opts.label != "" {
		return fmt.Sprintf("%s:label:%s", name, opts.label)
	}
	if opts.version > 0 {
		return fmt.Sprintf("%s:v%d", name, opts.version)
	}
	return fmt.Sprintf("%s:latest", name)
}

// PromptCache implements a simple TTL cache for prompts
type PromptCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
}

type cacheItem struct {
	prompt    *Prompt
	expiresAt time.Time
}

// NewPromptCache creates a new prompt cache
func NewPromptCache(ttl time.Duration) *PromptCache {
	cache := &PromptCache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a prompt from cache
func (c *PromptCache) Get(key string) *Prompt {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, ok := c.items[key]
	if !ok {
		return nil
	}
	
	if time.Now().After(item.expiresAt) {
		return nil // Expired
	}
	
	return item.prompt
}

// Set stores a prompt in cache
func (c *PromptCache) Set(key string, prompt *Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = &cacheItem{
		prompt:    prompt,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// InvalidatePrefix removes all cache entries with the given prefix
func (c *PromptCache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
		}
	}
}

// cleanup periodically removes expired items
func (c *PromptCache) cleanup() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// Helper function to safely get string from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TextPrompt is a helper to create a text prompt
func TextPrompt(name string, template string) *Prompt {
	return &Prompt{
		Name:    name,
		Type:    PromptTypeText,
		Prompt:  template,
		Config:  make(map[string]interface{}),
		Version: 1,
	}
}

// ChatPrompt is a helper to create a chat prompt
func ChatPrompt(name string, messages []ChatMessage) *Prompt {
	return &Prompt{
		Name:    name,
		Type:    PromptTypeChat,
		Prompt:  messages,
		Config:  make(map[string]interface{}),
		Version: 1,
	}
}

// Convenience methods on Langfuse client

// GetPrompt retrieves a prompt (convenience method)
func (l *Langfuse) GetPrompt(ctx context.Context, name string, opts ...PromptOption) (*Prompt, error) {
	pc := l.NewPromptClient()
	return pc.GetPrompt(ctx, name, opts...)
}

// CreatePrompt creates a new prompt (convenience method)
func (l *Langfuse) CreatePrompt(ctx context.Context, prompt *Prompt) (*Prompt, error) {
	pc := l.NewPromptClient()
	return pc.CreatePrompt(ctx, prompt)
}

// PromptTemplate provides a builder interface for prompts
type PromptTemplate struct {
	prompt *Prompt
}

// NewPromptTemplate creates a new prompt template builder
func NewPromptTemplate(name string) *PromptTemplate {
	return &PromptTemplate{
		prompt: &Prompt{
			Name:   name,
			Config: make(map[string]interface{}),
		},
	}
}

// AsText sets the prompt as a text type
func (pt *PromptTemplate) AsText(template string) *PromptTemplate {
	pt.prompt.Type = PromptTypeText
	pt.prompt.Prompt = template
	return pt
}

// AsChat sets the prompt as a chat type
func (pt *PromptTemplate) AsChat(messages []ChatMessage) *PromptTemplate {
	pt.prompt.Type = PromptTypeChat
	pt.prompt.Prompt = messages
	return pt
}

// WithConfig adds configuration parameters
func (pt *PromptTemplate) WithConfig(key string, value interface{}) *PromptTemplate {
	pt.prompt.Config[key] = value
	return pt
}

// WithLabels adds labels
func (pt *PromptTemplate) WithLabels(labels ...string) *PromptTemplate {
	pt.prompt.Labels = labels
	return pt
}

// Build returns the constructed prompt
func (pt *PromptTemplate) Build() *Prompt {
	return pt.prompt
}
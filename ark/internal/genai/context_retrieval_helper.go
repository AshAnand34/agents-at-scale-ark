/* Copyright 2025. McKinsey & Company */

package genai

import (
	"context"
	"fmt"
	"strings"

	arkv1alpha1 "mckinsey.com/ark/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ContextHelper handles contextual background information extraction for evaluations
type ContextHelper struct {
	client client.Client
}

// NewContextHelper creates a new context helper
func NewContextHelper(client client.Client) *ContextHelper {
	return &ContextHelper{
		client: client,
	}
}

// ExtractContextualBackground extracts only true contextual background information for evaluation
// This focuses on information that helps understand the user's input/query, not system configuration
func (h *ContextHelper) ExtractContextualBackground(ctx context.Context, evaluation *arkv1alpha1.Evaluation) (string, string) {
	log := logf.FromContext(ctx)

	switch evaluation.Spec.Type {
	case "query":
		if evaluation.Spec.Config.QueryRef != nil {
			return h.extractQueryContextualBackground(ctx, evaluation.Spec.Config.QueryRef, evaluation.Namespace)
		}
	case "direct":
		// Direct evaluations should have context provided via parameters
		log.Info("Direct evaluation - context should be provided via parameters")
	default:
		log.Info("Unknown evaluation type for context extraction", "type", evaluation.Spec.Type)
	}

	return "", "none"
}

// extractQueryContextualBackground extracts only true contextual background information from a query
func (h *ContextHelper) extractQueryContextualBackground(ctx context.Context, queryRef *arkv1alpha1.QueryRef, defaultNamespace string) (string, string) {
	log := logf.FromContext(ctx)

	// Determine namespace
	queryNamespace := queryRef.Namespace
	if queryNamespace == "" {
		queryNamespace = defaultNamespace
	}

	// Fetch the query
	var query arkv1alpha1.Query
	queryKey := client.ObjectKey{
		Name:      queryRef.Name,
		Namespace: queryNamespace,
	}

	if err := h.client.Get(ctx, queryKey, &query); err != nil {
		log.Error(err, "Failed to fetch query for context extraction", "queryName", queryRef.Name)
		return "", "none"
	}

	var contextBuilder strings.Builder
	contextSource := "none"
	hasContext := false

	// Extract conversation history from memory (true background context)
	if query.Spec.Memory != nil && query.Spec.Memory.Name != "" {
		memoryContext, memorySource := h.extractMemoryContext(ctx, query.Spec.Memory, query.Namespace)
		if memoryContext != "" {
			contextBuilder.WriteString(memoryContext)
			contextSource = memorySource
			hasContext = true
		}
	}

	// Extract contextual parameters (filter for actual context, not configuration)
	if len(query.Spec.Parameters) > 0 {
		contextualParams := h.filterContextualParameters(query.Spec.Parameters)
		if len(contextualParams) > 0 {
			if hasContext {
				contextBuilder.WriteString("\nAdditional Context:\n")
			} else {
				contextBuilder.WriteString("Context:\n")
			}

			for key, value := range contextualParams {
				contextBuilder.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
			}

			if hasContext {
				contextSource += "_with_params"
			} else {
				contextSource = "parameters"
				hasContext = true
			}
		}
	}

	if !hasContext {
		log.Info("No contextual background information found", "queryName", query.Name)
		return "", "none"
	}

	extractedContext := contextBuilder.String()
	log.Info("Extracted contextual background information",
		"queryName", query.Name,
		"contextLength", len(extractedContext),
		"contextSource", contextSource)

	return extractedContext, contextSource
}

// extractMemoryContext extracts conversation history (true background context)
func (h *ContextHelper) extractMemoryContext(ctx context.Context, memoryRef *arkv1alpha1.MemoryRef, defaultNamespace string) (string, string) {
	log := logf.FromContext(ctx)

	// Determine namespace
	memoryNamespace := memoryRef.Namespace
	if memoryNamespace == "" {
		memoryNamespace = defaultNamespace
	}

	// Fetch memory resource
	var memory arkv1alpha1.Memory
	memoryKey := client.ObjectKey{
		Name:      memoryRef.Name,
		Namespace: memoryNamespace,
	}

	if err := h.client.Get(ctx, memoryKey, &memory); err != nil {
		log.Error(err, "Failed to fetch memory for context extraction", "memoryName", memoryRef.Name)
		return "", "none"
	}

	// Memory CRD only tracks address, actual conversation content is in external service
	// For now, we note that conversation history exists at this address
	// TODO: In future, could fetch actual conversation content from memory service
	if memory.Status.LastResolvedAddress != nil && *memory.Status.LastResolvedAddress != "" {
		context := fmt.Sprintf("Previous conversation history available (stored at: %s)\n", *memory.Status.LastResolvedAddress)
		log.Info("Memory context extracted", "memoryName", memoryRef.Name, "address", *memory.Status.LastResolvedAddress)
		return context, "memory"
	}

	log.Info("Memory resource found but no conversation history available", "memoryName", memoryRef.Name)
	return "", "none"
}

// filterContextualParameters filters parameters to only include actual contextual information
// This excludes model configurations and includes only background information that helps understand the query
func (h *ContextHelper) filterContextualParameters(params []arkv1alpha1.Parameter) map[string]string {
	contextualParams := make(map[string]string)

	// Define patterns for contextual parameters (background information)
	contextualPrefixes := []string{
		"context",
		"background",
		"reference",
		"document",
		"history",
		"previous",
		"retrieved",
		"knowledge",
		"source",
		"material",
	}

	// Define patterns for non-contextual (configuration) parameters
	configPrefixes := []string{
		"model.",
		"temperature",
		"max_tokens",
		"top_p",
		"frequency_penalty",
		"presence_penalty",
		"langfuse.",
		"azure_",
		"openai_",
		"anthropic_",
		"google_",
		"ollama_",
		"timeout",
		"retry",
		"endpoint",
		"api_",
		"auth",
		"token",
		"key",
		"secret",
		"threshold",
		"metrics",
		"evaluation.",
	}

	for _, param := range params {
		paramName := strings.ToLower(param.Name)

		// Skip configuration parameters
		isConfig := false
		for _, prefix := range configPrefixes {
			if strings.HasPrefix(paramName, prefix) {
				isConfig = true
				break
			}
		}
		if isConfig {
			continue
		}

		// Include contextual parameters
		isContextual := false
		for _, prefix := range contextualPrefixes {
			if strings.HasPrefix(paramName, prefix) || strings.Contains(paramName, prefix) {
				isContextual = true
				break
			}
		}

		if isContextual && param.Value != "" {
			contextualParams[param.Name] = param.Value
		}
	}

	return contextualParams
}

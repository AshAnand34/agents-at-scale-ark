# ARK Evaluator

Unified AI evaluation service supporting both ARK's native LLM-as-a-Judge and external OSS platforms.

### Quick Build 
```bash
# From project root
make ark-evaluator-build
```

### Full Workflow
```bash
# From project root
make ark-evaluator-deps     # Install dependencies (including ark-sdk)
make ark-evaluator-test     # Run tests
make ark-evaluator-build    # Build Docker image
make ark-evaluator-install  # Deploy to cluster
kubectl rollout restart deployment ark-evaluator  #force deployment update
```

### Development
```bash
make ark-evaluator-dev      # Run service locally
```

## Evaluation Providers

The service supports multiple evaluation providers:

### ARK Native Evaluation (Default)
- **Types**: direct, query, baseline, batch, event
- **No additional dependencies required**
- **Uses LLM-as-a-Judge approach**

### OSS Platform Providers
- **Langfuse**: Tracing and evaluation platform (`provider: langfuse`)
- **RAGAS**: RAG-specific metrics (placeholder)
- **UpTrain**: Comprehensive AI evaluation (placeholder)
- **Opik**: Comet's evaluation platform (placeholder)

## Configuration

The service automatically receives model configuration from the Ark Evaluator custom resource, supporting:

- OpenAI-compatible APIs
- Azure OpenAI services
- Custom API endpoints

## Evaluation Criteria

The service evaluates responses on:

1. **Relevance** (0-1): How well responses address the query
2. **Accuracy** (0-1): Factual correctness and reliability
3. **Completeness** (0-1): Comprehensiveness of information
4. **Clarity** (0-1): Readability and understanding
5. **Usefulness** (0-1): Practical value to the user

Responses with scores ≥0.7 are marked as "passed".
## Usage Examples

### ARK Native Evaluation
```yaml
parameters:
  # No provider specified - uses ARK native
  scope: "all"
  min_score: "0.7"
```

### Langfuse Integration
```yaml
parameters:
  provider: "langfuse"
  langfuse.host: "https://cloud.langfuse.com"
  langfuse.public_key: "pk-lf-xxx"
  langfuse.secret_key: "sk-lf-xxx"
```

See `examples/` directory for complete configuration examples.

## Notes
- Requires Python with uv package manager
- ARK native evaluation requires no additional dependencies
- Backward compatible with existing ARK evaluations

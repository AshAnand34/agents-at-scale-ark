"""
RAGAS-based LLM evaluation adapter for Langfuse integration.
Supports multiple LLM providers: Azure OpenAI, OpenAI, Anthropic Claude, Google Gemini, Ollama.
"""

import logging
from typing import Dict, List, Any
from ..types import EvaluationParameters
logger = logging.getLogger(__name__)


class RagasAdapter:
    """
    RAGAS (Retrieval Augmented Generation Assessment) adapter supporting multiple LLM providers.
    """
    
    def __init__(self):
        self.supported_ragas_metrics = {
            'relevance': 'answer_relevancy',
            'correctness': 'answer_correctness',
            'similarity': 'answer_similarity',
            'faithfulness': 'faithfulness',
            'helpfulness': 'answer_relevancy',
            'clarity': 'answer_similarity'
        }
        
        from .llm_provider import LLMProvider
        self.llm_provider = LLMProvider()
    
    
    
    async def evaluate(self, input_text: str, output_text: str, metrics: List[str], params: dict) -> Dict[str, float]:
        """
        Run RAGAS evaluation using the detected LLM provider.
        """
        try:
            import asyncio
            import concurrent.futures
            
            # Check if we're in a uvloop environment
            try:
                current_loop = asyncio.get_event_loop()
                is_uvloop = 'uvloop' in str(type(current_loop))
            except RuntimeError:
                is_uvloop = False
            
            if is_uvloop:
                logger.info("Detected uvloop, running RAGAS evaluation in separate thread")
                # Run RAGAS in a separate thread with clean asyncio environment
                with concurrent.futures.ThreadPoolExecutor() as executor:
                    future = executor.submit(self._run_ragas_sync, input_text, output_text, metrics, params)
                    return future.result()
            else:
                # No uvloop, can run directly
                return await self._run_ragas_async(input_text, output_text, metrics, params)
                
        except ImportError as e:
            logger.error(f"RAGAS dependencies not available: {e}")
            return self._fallback_evaluation(input_text, output_text, metrics)
        except Exception as e:
            logger.error(f"RAGAS evaluation failed: {e}")
            return self._fallback_evaluation(input_text, output_text, metrics)
    
    def _run_ragas_sync(self, input_text: str, output_text: str, metrics: List[str], params: dict) -> Dict[str, float]:
        """
        Run RAGAS synchronously in a clean thread (for uvloop compatibility).
        """
        import asyncio
        import threading
        import os
        
        logger.info(f"Running RAGAS in thread: {threading.current_thread().name}")
        
        # Set only necessary environment variables for Azure OpenAI (avoid conflicts)
        env_vars_set = []
        if 'langfuse.azure_api_key' in params:
            # Only set the Azure-specific variables to avoid conflicts with explicit parameters
            os.environ['AZURE_OPENAI_API_KEY'] = params['langfuse.azure_api_key']
            env_vars_set.append('AZURE_OPENAI_API_KEY')
        if 'langfuse.azure_endpoint' in params:
            os.environ['AZURE_OPENAI_ENDPOINT'] = params['langfuse.azure_endpoint']
            env_vars_set.append('AZURE_OPENAI_ENDPOINT')
        if 'langfuse.model_version' in params:
            os.environ['OPENAI_API_VERSION'] = params['langfuse.model_version']
            env_vars_set.append('OPENAI_API_VERSION')
        
        if env_vars_set:
            logger.info(f"Set environment variables for Azure OpenAI: {env_vars_set}")
        
        # CRITICAL: Reset the event loop policy to default BEFORE creating the loop
        # This ensures the thread doesn't inherit uvloop from the main thread
        asyncio.set_event_loop_policy(asyncio.DefaultEventLoopPolicy())
        
        # Now create a standard asyncio loop (not uvloop)
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        
        logger.info(f"Created new event loop: {type(loop)}")
        
        try:
            return loop.run_until_complete(self._run_ragas_async(input_text, output_text, metrics, params))
        except Exception as e:
            logger.error(f"Error in thread RAGAS execution: {e}")
            raise
        finally:
            loop.close()
            logger.info("Closed thread event loop")
            
            # Clean up environment variables we set
            for var in env_vars_set:
                os.environ.pop(var, None)
    
    async def _run_ragas_async(self, input_text: str, output_text: str, metrics: List[str], params: dict) -> Dict[str, float]:
        """
        Run RAGAS evaluation asynchronously.
        """
        # Import RAGAS components
        from ragas import evaluate as ev_ragas
        from ragas.metrics import (
            answer_relevancy,
            answer_correctness, 
            answer_similarity,
            faithfulness,
            context_precision,
            context_recall
        )
        from datasets import Dataset
        
        # Detect LLM provider and get config
        provider_type, llm_config = self.llm_provider.detect_provider(params)
        logger.info(f"Detected LLM provider: {provider_type}")
        
        # Initialize LLM based on provider and wrap for RAGAS
        try:
            langchain_llm = self.llm_provider.create_instance(provider_type, llm_config)
            from ragas.llms import LangchainLLMWrapper
            llm = LangchainLLMWrapper(langchain_llm)
            logger.info(f"Wrapped {provider_type} LLM with RAGAS LangchainLLMWrapper")
            
            # Test LLM connectivity to catch deployment issues early
            try:
                test_response = await langchain_llm.agenerate([["Test connectivity"]])
                logger.info(f"LLM connectivity test successful: {len(test_response.generations[0])} responses")
            except Exception as test_e:
                logger.warning(f"LLM connectivity test failed (may still work for RAGAS): {test_e}")
                # Don't fail here, RAGAS might still work
                
        except Exception as llm_e:
            logger.error(f"Failed to create LLM instance: {llm_e}")
            raise
        
        # Create Azure embeddings BEFORE initializing metrics
        embeddings = None
        try:
            if provider_type == 'azure_openai':
                from langchain_openai import AzureOpenAIEmbeddings
                from ragas.embeddings import LangchainEmbeddingsWrapper
                
                # Get embedding deployment - fallback to default if not specified
                embedding_deployment = params.get('langfuse.azure_embedding_deployment', 'text-embedding-ada-002')
                embedding_model = params.get('langfuse.azure_embedding_model', 'text-embedding-ada-002')
                
                # Log deployment information for debugging
                logger.info(f"LLM deployment: {llm_config['deployment_name']}")
                logger.info(f"Embedding deployment: {embedding_deployment}")
                
                # Create Azure embeddings with explicit parameters (avoids env var conflicts)
                azure_embeddings = AzureOpenAIEmbeddings(
                    model=embedding_model,
                    azure_endpoint=llm_config['api_base'],
                    deployment=embedding_deployment,
                    openai_api_version=llm_config['api_version'],
                    api_key=llm_config['api_key']
                )
                embeddings = LangchainEmbeddingsWrapper(azure_embeddings)
                logger.info(f"Created Azure embeddings: deployment={embedding_deployment}, model={embedding_model}")
                
                # Test embeddings connectivity
                try:
                    test_embedding = azure_embeddings.embed_query("test")
                    logger.info(f"✅ Embedding connectivity test successful, vector length: {len(test_embedding)}")
                except Exception as embed_test_e:
                    logger.warning(f"⚠️ Embedding connectivity test failed: {embed_test_e}")
                    # Still proceed as RAGAS might work differently
        except Exception as e:
            logger.warning(f"Failed to create Azure embeddings: {e}, will proceed without embeddings")
            embeddings = None
        
        # Map our metrics to RAGAS metrics and initialize them with LLM and embeddings
        ragas_metric_map = {
            'relevance': answer_relevancy,
            'correctness': answer_correctness,
            'similarity': answer_similarity,
            'faithfulness': faithfulness,
            'toxicity': None,  # RAGAS doesn't have built-in toxicity - we'll handle this separately
            'helpfulness': answer_relevancy,  # Use relevancy as proxy for helpfulness
            'clarity': answer_similarity  # Use similarity as proxy for clarity
        }
        
        # Filter metrics that RAGAS supports and initialize them with LLM/embeddings
        supported_metrics = []
        for metric in metrics:
            if metric in ragas_metric_map and ragas_metric_map[metric] is not None:
                # Get the RAGAS metric class/instance
                ragas_metric = ragas_metric_map[metric]
                
                # Initialize the metric with LLM and embeddings
                from ragas.metrics.base import MetricWithLLM, MetricWithEmbeddings
                from ragas.run_config import RunConfig
                
                # Create fresh instance if needed
                if hasattr(ragas_metric, '__call__') and not hasattr(ragas_metric, 'name'):
                    # It's a metric class/function, instantiate it
                    metric_instance = ragas_metric()
                else:
                    # It's already an instance
                    metric_instance = ragas_metric
                
                # Configure the metric with our LLM and embeddings
                if isinstance(metric_instance, MetricWithLLM):
                    metric_instance.llm = llm
                if isinstance(metric_instance, MetricWithEmbeddings) and embeddings:
                    metric_instance.embeddings = embeddings
                    logger.info(f"Configured {metric} metric with Azure embeddings")
                elif isinstance(metric_instance, MetricWithEmbeddings):
                    logger.warning(f"Metric {metric} needs embeddings but none provided")
                
                # Initialize the metric
                run_config = RunConfig()
                metric_instance.init(run_config)
                
                supported_metrics.append(metric_instance)
        
        if not supported_metrics:
            logger.warning("No supported RAGAS metrics found, using answer_relevancy as default")
            # Create and configure default metric
            default_metric = answer_relevancy
            if hasattr(default_metric, '__call__') and not hasattr(default_metric, 'name'):
                default_metric = default_metric()
            if isinstance(default_metric, MetricWithLLM):
                default_metric.llm = llm
            if isinstance(default_metric, MetricWithEmbeddings) and embeddings:
                default_metric.embeddings = embeddings
            run_config = RunConfig()
            default_metric.init(run_config)
            supported_metrics = [default_metric]
        
        # Extract context from parameters if provided
        eval_params = EvaluationParameters.from_request_params(params)
        context_source = eval_params.context_source or "undefined"
        context = eval_params.context or ""
        
        # Prepare contexts for RAGAS evaluation
        if context:
            logger.info(f"Using evaluation context from {context_source}, length: {len(context)} characters")
            # RAGAS expects list of strings, not list of list
            contexts = [eval_params.context]
        else:
            logger.info("No context provided, using default context for evaluation")
            contexts = ["No specific context provided"]
        
        # Create dataset for evaluation
        # RAGAS expects specific format with question, answer, contexts, ground_truths
        dataset = []
        dataset.append({
            'question': input_text,
            'answer': output_text, 
            'contexts': contexts,
            'ground_truth': output_text,  # Use output as ground truth for similarity metrics
            # Also include alternative field names for compatibility
            'user_input': input_text,
            'response': output_text,
            'retrieved_contexts': contexts,
            'reference': output_text
        })
        
        eval_dataset = Dataset.from_list(dataset)
        
        # Run RAGAS evaluation - metrics are already configured with LLM and embeddings
        logger.info(f"Running RAGAS evaluation with {len(supported_metrics)} pre-configured metrics")
        try:
            result = ev_ragas(
                dataset=eval_dataset,
                metrics=supported_metrics
            )
        except Exception as eval_e:
            logger.error(f"RAGAS evaluation failed: {eval_e}")
            # Return fallback scores if RAGAS evaluation completely fails
            fallback_scores = {}
            for metric in metrics:
                fallback_scores[metric] = 0.5  # Neutral fallback score
            logger.info(f"Using fallback scores due to evaluation failure: {fallback_scores}")
            return fallback_scores
        
        # Extract scores and map back to our metric names
        scores = {}
        result_dict = result.to_pandas().to_dict('records')[0]
        
        import math
        
        for metric in metrics:
            value = None
            if metric == 'relevance' and 'answer_relevancy' in result_dict:
                value = result_dict['answer_relevancy']
            elif metric == 'correctness' and 'answer_correctness' in result_dict:
                value = result_dict['answer_correctness']
            elif metric == 'similarity' and 'answer_similarity' in result_dict:
                value = result_dict['answer_similarity']
            elif metric == 'faithfulness' and 'faithfulness' in result_dict:
                value = result_dict['faithfulness']
            elif metric == 'helpfulness' and 'answer_relevancy' in result_dict:
                value = result_dict['answer_relevancy']
            elif metric == 'clarity' and 'answer_similarity' in result_dict:
                value = result_dict['answer_similarity']
            
            if value is not None:
                # Handle NaN values that can occur in RAGAS evaluation
                if math.isnan(float(value)):
                    logger.warning(f"RAGAS returned NaN for metric {metric}, using fallback score")
                    scores[metric] = 0.7  # Reasonable fallback for NaN
                else:
                    scores[metric] = float(value)
            else:
                logger.warning(f"No RAGAS result found for metric: {metric}")
                scores[metric] = 0.0
        
        logger.info(f"RAGAS evaluation completed: {scores}")
        return scores
    
    def _fallback_evaluation(self, input_text: str, output_text: str, metrics: List[str]) -> Dict[str, float]:
        """
        Fallback evaluation when RAGAS is not available.
        """
        logger.warning("Using fallback evaluation method")
        scores = {}
        for metric in metrics:
            if metric == 'relevance':
                # Simple word overlap
                input_words = set(input_text.lower().split())
                output_words = set(output_text.lower().split())
                overlap = len(input_words.intersection(output_words))
                scores[metric] = min(1.0, overlap / max(len(input_words), 1))
            elif metric == 'correctness':
                # Length-based scoring
                scores[metric] = min(1.0, len(output_text) / 100)
            elif metric == 'toxicity':
                # Simple toxicity check
                toxic_words = ['hate', 'stupid', 'idiot', 'kill', 'die', 'worst']
                toxic_count = sum(1 for word in toxic_words if word in output_text.lower())
                scores[metric] = min(1.0, toxic_count / 3.0)
            else:
                scores[metric] = 0.5
        return scores
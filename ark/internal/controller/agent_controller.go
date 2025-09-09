/* Copyright 2025. McKinsey & Company */

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	arkv1alpha1 "mckinsey.com/ark/api/v1alpha1"
)

type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ark.mckinsey.com,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ark.mckinsey.com,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ark.mckinsey.com,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=ark.mckinsey.com,resources=tools,verbs=get;list;watch
// +kubebuilder:rbac:groups=ark.mckinsey.com,resources=models,verbs=get;list;watch

func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Agent instance
	var agent arkv1alpha1.Agent
	if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Agent resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Agent")
		return ctrl.Result{}, err
	}

	// Initialize phase to Pending if not set (for newly created agents)
	if agent.Status.Phase == "" {
		agent.Status.Phase = arkv1alpha1.AgentPhasePending
		if err := r.Status().Update(ctx, &agent); err != nil {
			log.Error(err, "Failed to initialize Agent status")
			return ctrl.Result{}, err
		}
		log.Info("Initialized agent status to Pending", "agent", agent.Name)
	}

	// Check tool dependencies and update status
	newPhase, err := r.checkDependencies(ctx, &agent)
	if err != nil {
		log.Error(err, "Failed to check dependencies")
		return ctrl.Result{}, err
	}

	// Update status if phase changed
	if agent.Status.Phase != newPhase {
		agent.Status.Phase = newPhase
		if err := r.Status().Update(ctx, &agent); err != nil {
			log.Error(err, "Failed to update Agent status")
			// Return error to trigger retry - controller-runtime will handle the retry with backoff
			return ctrl.Result{}, err
		}
		log.Info("Updated agent status", "phase", newPhase)
	}

	// Requeue if still pending to check for dependency resolution
	// Note: We also watch for Tool/Model events, so this is just a fallback
	if newPhase == arkv1alpha1.AgentPhasePending {
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil // Reduced frequency since we have event-driven updates
	}

	return ctrl.Result{}, nil
}

// checkDependencies validates all agent dependencies and returns appropriate phase
func (r *AgentReconciler) checkDependencies(ctx context.Context, agent *arkv1alpha1.Agent) (arkv1alpha1.AgentPhase, error) {
	// Check model dependency
	if phase, err := r.checkModelDependency(ctx, agent); err != nil || phase != arkv1alpha1.AgentPhaseRunning {
		return phase, err
	}

	// Check tool dependencies
	return r.checkToolDependencies(ctx, agent)
}

// checkModelDependency validates model dependency
func (r *AgentReconciler) checkModelDependency(ctx context.Context, agent *arkv1alpha1.Agent) (arkv1alpha1.AgentPhase, error) {
	if agent.Spec.ModelRef == nil {
		return arkv1alpha1.AgentPhaseRunning, nil
	}

	log := logf.FromContext(ctx)
	modelNamespace := agent.Spec.ModelRef.Namespace
	if modelNamespace == "" {
		modelNamespace = agent.Namespace
	}

	var model arkv1alpha1.Model
	modelKey := types.NamespacedName{Name: agent.Spec.ModelRef.Name, Namespace: modelNamespace}
	if err := r.Get(ctx, modelKey, &model); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Model dependency not found", "model", agent.Spec.ModelRef.Name, "namespace", modelNamespace)
			return arkv1alpha1.AgentPhasePending, nil
		}
		return arkv1alpha1.AgentPhaseUnknown, err
	}

	return arkv1alpha1.AgentPhaseRunning, nil
}

// checkToolDependencies validates tool dependencies
func (r *AgentReconciler) checkToolDependencies(ctx context.Context, agent *arkv1alpha1.Agent) (arkv1alpha1.AgentPhase, error) {
	log := logf.FromContext(ctx)

	for _, toolSpec := range agent.Spec.Tools {
		if toolSpec.Type == "custom" && toolSpec.Name != "" {
			var tool arkv1alpha1.Tool
			toolKey := types.NamespacedName{Name: toolSpec.Name, Namespace: agent.Namespace}
			if err := r.Get(ctx, toolKey, &tool); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Tool dependency not found", "tool", toolSpec.Name, "namespace", agent.Namespace)
					return arkv1alpha1.AgentPhasePending, nil
				}
				return arkv1alpha1.AgentPhaseUnknown, err
			}
		}
	}

	// All dependencies resolved
	return arkv1alpha1.AgentPhaseRunning, nil
}

func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&arkv1alpha1.Agent{}).
		// Watch for Tool events and reconcile dependent agents
		Watches(
			&arkv1alpha1.Tool{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsForTool),
		).
		// Watch for Model events and reconcile dependent agents
		Watches(
			&arkv1alpha1.Model{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsForModel),
		).
		Named("agent").
		Complete(r)
}

// findAgentsForTool finds agents that depend on the given tool
func (r *AgentReconciler) findAgentsForTool(ctx context.Context, obj client.Object) []reconcile.Request {
	tool, ok := obj.(*arkv1alpha1.Tool)
	if !ok {
		return nil
	}

	return r.findAgentsForDependency(ctx, tool.Name, tool.Namespace, "tool", func(agent *arkv1alpha1.Agent) bool {
		return r.agentDependsOnTool(agent, tool.Name)
	})
}

// findAgentsForModel finds agents that depend on the given model
func (r *AgentReconciler) findAgentsForModel(ctx context.Context, obj client.Object) []reconcile.Request {
	model, ok := obj.(*arkv1alpha1.Model)
	if !ok {
		return nil
	}

	return r.findAgentsForDependency(ctx, model.Name, model.Namespace, "model", func(agent *arkv1alpha1.Agent) bool {
		return r.agentDependsOnModel(agent, model.Name)
	})
}

// findAgentsForDependency is a generic function to find agents that depend on a given resource
func (r *AgentReconciler) findAgentsForDependency(ctx context.Context, resourceName, namespace, resourceType string, dependencyCheck func(*arkv1alpha1.Agent) bool) []reconcile.Request {
	log := logf.Log.WithName("agent-controller").WithValues(resourceType, resourceName, "namespace", namespace)

	// List all agents in the same namespace
	var agentList arkv1alpha1.AgentList
	if err := r.List(ctx, &agentList, client.InNamespace(namespace)); err != nil {
		log.Error(err, "Failed to list agents for dependency check", "resourceType", resourceType)
		return nil
	}

	var requests []reconcile.Request
	seenAgents := make(map[string]bool) // Deduplication map

	for _, agent := range agentList.Items {
		// Check if this agent depends on the resource
		if dependencyCheck(&agent) {
			agentKey := agent.Namespace + "/" + agent.Name

			// Skip if we've already added this agent
			if seenAgents[agentKey] {
				continue
			}
			seenAgents[agentKey] = true

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      agent.Name,
					Namespace: agent.Namespace,
				},
			})
			log.Info("Triggering reconciliation for agent dependent on resource", "agent", agent.Name, "resourceType", resourceType)
		}
	}

	return requests
}

// agentDependsOnTool checks if an agent depends on a specific tool
func (r *AgentReconciler) agentDependsOnTool(agent *arkv1alpha1.Agent, toolName string) bool {
	for _, toolSpec := range agent.Spec.Tools {
		if toolSpec.Type == "custom" && toolSpec.Name == toolName {
			return true
		}
	}
	return false
}

// agentDependsOnModel checks if an agent depends on a specific model
func (r *AgentReconciler) agentDependsOnModel(agent *arkv1alpha1.Agent, modelName string) bool {
	return agent.Spec.ModelRef != nil && agent.Spec.ModelRef.Name == modelName
}

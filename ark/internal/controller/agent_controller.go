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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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
			return ctrl.Result{}, err
		}
		log.Info("Updated agent status", "phase", newPhase)
	}

	// Requeue if still pending to check for dependency resolution
	if newPhase == arkv1alpha1.AgentPhasePending {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
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
		Named("agent").
		Complete(r)
}

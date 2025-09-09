# Agent Status Test Script

Tests the event-driven agent dependency resolution system.
This validates the event-driven architecture replacing blocking agent creation dependency resolution.

## Prerequisites

```bash
make quickstart
```

## Usage

```bash
# Run all scenarios
./test-nonblocking-agent-status.sh

# Run individual scenarios
./test-nonblocking-agent-status.sh scenario1  # Agent without dependencies (Pending)
./test-nonblocking-agent-status.sh scenario2  # Add tool (→ Running)
./test-nonblocking-agent-status.sh scenario3  # Remove tool (→ Pending)

# Cleanup
./test-nonblocking-agent-status.sh cleanup
```

## Expected Results

- ✅ Agent shows `Pending` when tool dependency is missing
- ✅ Agent transitions to `Running` immediately when tool is created
- ✅ Agent transitions back to `Pending` immediately when tool is deleted

## Troubleshooting

```bash
# Check controller logs
kubectl logs -n ark-system deployment/ark-controller --follow

# Verify controller deployment
kubectl get deployments -n ark-system

# Check agent status
kubectl get agent foo-agent -o yaml
```


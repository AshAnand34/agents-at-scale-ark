#!/bin/bash

# Test script for agent status changes based on tool dependencies
# This script demonstrates the event-driven dependency resolution system

set -e

AGENT_NAME="foo-agent"
TOOL_NAME="nonexisting-tool"
NAMESPACE="default"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_separator() {
    echo -e "\n${BLUE}============================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}============================================${NC}\n"
}

# Function to get agent status
get_agent_status() {
    kubectl get agent "$AGENT_NAME" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound"
}

# Function to wait for status change
wait_for_status() {
    local expected_status="$1"
    local timeout="${2:-30}"
    local count=0
    
    print_info "Waiting for agent status to become '$expected_status' (timeout: ${timeout}s)..."
    
    while [ $count -lt $timeout ]; do
        local current_status=$(get_agent_status)
        if [ "$current_status" = "$expected_status" ]; then
            print_success "Agent status is now: $current_status"
            return 0
        fi
        echo -n "."
        sleep 1
        count=$((count + 1))
    done
    
    echo ""
    print_error "Timeout waiting for status '$expected_status'. Current status: $(get_agent_status)"
    return 1
}

# Function to show agent details
show_agent_details() {
    print_info "Current agent details:"
    kubectl get agent "$AGENT_NAME" -n "$NAMESPACE" -o yaml 2>/dev/null | grep -A 10 -B 5 "status:\|phase:\|tools:" || print_warning "Agent not found"
    echo ""
}

# Function to show tool status
show_tool_status() {
    print_info "Tool status:"
    if kubectl get tool "$TOOL_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
        print_success "Tool '$TOOL_NAME' exists"
        kubectl get tool "$TOOL_NAME" -n "$NAMESPACE" -o yaml | grep -A 5 -B 5 "metadata:\|spec:\|status:" | head -20
    else
        print_warning "Tool '$TOOL_NAME' does not exist"
    fi
    echo ""
}

# Function to create test agent
create_test_agent() {
    print_info "Creating test agent '$AGENT_NAME' with dependency on tool '$TOOL_NAME'..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: ark.mckinsey.com/v1alpha1
kind: Agent
metadata:
  name: $AGENT_NAME
  namespace: $NAMESPACE
spec:
  description: "Test agent for dependency resolution"
  prompt: "You are a test agent"
  tools:
    - type: custom
      name: $TOOL_NAME
EOF

    if [ $? -eq 0 ]; then
        print_success "Agent created successfully"
    else
        print_error "Failed to create agent"
        exit 1
    fi
}

# Function to create test tool
create_test_tool() {
    print_info "Creating test tool '$TOOL_NAME'..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: ark.mckinsey.com/v1alpha1
kind: Tool
metadata:
  name: $TOOL_NAME
  namespace: $NAMESPACE
spec:
  type: http
  description: "Test tool for dependency resolution"
  http:
    url: "https://httpbin.org/get"
    method: "GET"
EOF

    if [ $? -eq 0 ]; then
        print_success "Tool created successfully"
    else
        print_error "Failed to create tool"
        exit 1
    fi
}

# Function to delete tool
delete_test_tool() {
    print_info "Deleting test tool '$TOOL_NAME'..."
    kubectl delete tool "$TOOL_NAME" -n "$NAMESPACE" --ignore-not-found=true
    if [ $? -eq 0 ]; then
        print_success "Tool deleted successfully"
    fi
}

# Function to delete agent
delete_test_agent() {
    print_info "Deleting test agent '$AGENT_NAME'..."
    kubectl delete agent "$AGENT_NAME" -n "$NAMESPACE" --ignore-not-found=true
    if [ $? -eq 0 ]; then
        print_success "Agent deleted successfully"
    fi
}

# Function to monitor controller logs
monitor_controller_logs() {
    local duration="${1:-10}"
    print_info "Monitoring controller logs for ${duration} seconds..."
    echo "Ctrl+C to stop monitoring early"
    
    timeout $duration kubectl logs -n ark-system deployment/ark-controller --follow --tail=50 | \
        grep -E "(agent|tool|Tool|Agent|status|phase|dependency|reconcil)" || true
    echo ""
}

# Main test scenarios
run_scenario_1() {
    print_separator "SCENARIO 1: Create Agent without Tool (should be Pending)"
    
    cleanup
    sleep 2
    
    # Create agent without tool dependency
    create_test_agent
    sleep 3
    
    # Check status
    show_agent_details
    show_tool_status
    
    local status=$(get_agent_status)
    if [ "$status" = "Pending" ]; then
        print_success "✅ Agent correctly shows Pending status when tool is missing"
    else
        print_error "❌ Expected Pending status, got: $status"
    fi
}

run_scenario_2() {
    print_separator "SCENARIO 2: Add Missing Tool (should transition to Running)"

    cleanup
    sleep 2
    
    # Create agent without tool dependency
    create_test_agent
    sleep 3
    
    local monitor_pid=$!
    
    # Create the missing tool
    create_test_tool
    
    # Wait for status change
    if wait_for_status "Running" 15; then
        print_success "✅ Agent correctly transitioned to Running when tool was created"
    else
        print_error "❌ Agent did not transition to Running status"
    fi
    
    # Stop log monitoring
    kill $monitor_pid 2>/dev/null || true
    wait $monitor_pid 2>/dev/null || true
    
    show_agent_details
    show_tool_status
}

run_scenario_3() {
    print_separator "SCENARIO 3: Remove Tool (should transition back to Pending)"

    cleanup
    sleep 2
    
    # Create agent without tool dependency
    create_test_agent
    create_test_tool
    sleep 3
    
    local monitor_pid=$!
    
    # Delete the tool
    delete_test_tool
    
    # Wait for status change
    if wait_for_status "Pending" 15; then
        print_success "✅ Agent correctly transitioned back to Pending when tool was deleted"
    else
        print_error "❌ Agent did not transition back to Pending status"
    fi
    
    # Stop log monitoring
    kill $monitor_pid 2>/dev/null || true
    wait $monitor_pid 2>/dev/null || true
    
    show_agent_details
    show_tool_status
}

# Cleanup function
cleanup() {
    print_separator "CLEANUP"
    delete_test_tool
    delete_test_agent
    print_success "Cleanup completed"
}

# Main execution
main() {
    print_separator "ARK Agent Status Test Script"
    print_info "Testing agent: $AGENT_NAME"
    print_info "Testing tool: $TOOL_NAME"
    print_info "Namespace: $NAMESPACE"
    echo ""
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if we can connect to the cluster
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if ARK controller is running
    if ! kubectl get deployment ark-controller -n ark-system &> /dev/null; then
        print_error "ARK controller is not deployed in ark-system namespace"
        exit 1
    fi
    
    # Run test scenarios
    run_scenario_1
    sleep 3
    run_scenario_2
    sleep 3
    run_scenario_3
    
    # Final cleanup
    cleanup
    
    print_separator "TEST COMPLETED"
    print_success "All scenarios completed. Check the output above for results."
}

# Handle script interruption
trap cleanup EXIT

# Parse command line arguments
case "${1:-all}" in
    "cleanup")
        cleanup
        ;;
    "scenario1")
        run_scenario_1
        ;;
    "scenario2")
        run_scenario_2
        ;;
    "scenario3")
        run_scenario_3
        ;;
    "all"|*)
        main
        ;;
esac

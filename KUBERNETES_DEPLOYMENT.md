# ConureDB Kubernetes Deployment Guide

This guide covers deploying ConureDB in Kubernetes using Helm charts with proper raft cluster formation and arbiter-based quorum logic.

## Overview

ConureDB is a raft-backed key-value store with B-Tree storage. The Helm chart provides:

- **Smart scaling**: Automatically handles raft node removal during scale-down operations
- **Bootstrap node**: Always 1 replica, handles initial cluster bootstrap  
- **Voter nodes**: Configurable number of additional voting members
- **Arbiter node**: Automatically deployed when needed to maintain odd quorum size
- **Pod Disruption Budget**: Maintains cluster availability during rolling updates
- **Graceful shutdown**: PreStop hooks ensure nodes leave raft cluster cleanly

## Architecture

### Quorum Logic

The chart implements an arbiter-based quorum system:

- **Odd number of voters**: No arbiter needed (e.g., 3 voters = 3-node quorum)
- **Even number of voters**: 1 arbiter added (e.g., 2 voters + 1 arbiter = 3-node quorum)
- **Arbiter mode**:
  - `auto` (default): Creates arbiter only when voters count is even
  - `always`: Always creates an arbiter
  - `never`: Never creates an arbiter

### StatefulSets

1. **conure-bootstrap**: Single bootstrap node (conure-0) that initializes the cluster
2. **conure**: Additional voter nodes that join the existing cluster
3. **conure-arbiter**: Arbiter node (when needed) that participates in voting but stores minimal data

## Quick Start

### Prerequisites

- Kubernetes cluster
- Helm 3.x
- Storage class for persistent volumes

### Installation

```bash
# Install with default values (2 voters + 1 arbiter = 3-node cluster)
helm install conuredb ./charts/conuredb

# Install with custom configuration
helm install conuredb ./charts/conuredb \
  --set voters.replicas=3 \
  --set voters.pvc.size=20Gi \
  --set image.tag=latest
```

### Configuration

Key configuration options in `values.yaml`:

```yaml
# Number of voter nodes (including bootstrap)
voters:
  replicas: 2

# Arbiter configuration
arbiter:
  mode: auto  # auto | always | never

# Services
service:
  httpPort: 8081
  raftPort: 7001

# Cluster joining
join:
  seeds:
    - http://conure-0.conure-hs:8081
    - http://conure-1.conure-hs:8081
  backoffSeconds: 2
  retries: 0  # 0=infinite
```

## Fixes Implemented

### 1. Bootstrap Logic Fix

**Problem**: All voter pods were trying to bootstrap simultaneously.

**Solution**:

- Created separate `conure-bootstrap` StatefulSet for the bootstrap node
- Only `conure-0` runs with `--bootstrap=true`
- Other nodes run with `--bootstrap=false` and auto-join

### 2. Service Discovery Fix

**Problem**: Headless service only selected voter pods, excluding arbiters.

**Solution**:

- Removed role-specific selector from headless service
- All pods (voters + arbiters) are now discoverable via `conure-hs`

### 3. Startup Ordering Fix

**Problem**: Race conditions during cluster initialization.

**Solution**:

- Added init containers to wait for bootstrap node readiness
- Voter nodes wait for `conure-0` to be ready before starting
- Arbiter waits for at least one voter to be ready

### 4. Pod Disruption Budget Fix

**Problem**: PDB calculations needed to account for bootstrap + voters + arbiter.

**Solution**:

- Correctly calculates majority based on total cluster size
- Accounts for bootstrap node in total count

## Deployment Examples

### Small Cluster (3 nodes)

```bash
helm install conuredb ./charts/conuredb \
  --set voters.replicas=3 \
  --set arbiter.mode=never
```

Result: 1 bootstrap + 2 voters = 3-node cluster (no arbiter needed)

### Medium Cluster (5 nodes)

```bash
helm install conuredb ./charts/conuredb \
  --set voters.replicas=4 \
  --set arbiter.mode=auto
```

Result: 1 bootstrap + 3 voters + 1 arbiter = 5-node cluster

### Production Cluster

```bash
helm install conuredb ./charts/conuredb \
  --set voters.replicas=4 \
  --set voters.pvc.size=100Gi \
  --set voters.pvc.storageClassName=fast-ssd \
  --set arbiter.pvc.size=10Gi \
  --set security.runAsNonRoot=true \
  --set security.runAsUser=1000 \
  --set volumePermissions.enabled=true
```

## REST API Usage

ConureDB provides a comprehensive REST API for key-value operations and cluster management.

### Key-Value Operations

#### Store a Key-Value Pair
```bash
# Store a simple value
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl -X PUT "http://localhost:8081/kv?key=mykey&value=myvalue"

# Store JSON data
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl -X PUT "http://localhost:8081/kv?key=config&value=%7B%22db%22%3A%22prod%22%7D"

# Response: OK
```

#### Retrieve a Value
```bash
# Get a specific key
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl "http://localhost:8081/kv?key=mykey"

# Response: myvalue
```

#### Delete a Key
```bash
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl -X DELETE "http://localhost:8081/kv?key=mykey"

# Response: OK
```

### Cluster Management

#### Check Cluster Status
```bash
# Get leader information
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl "http://localhost:8081/status"

# Response: {"is_leader":true,"leader":"10.42.0.116:7001"}
```

#### View Cluster Configuration
```bash
# Get raft cluster members
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl "http://localhost:8081/raft/config"

# Response: 
# {
#   "leader": "10.42.0.116:7001",
#   "servers": [
#     {"id": "conure-bootstrap-0", "address": "10.42.0.116:7001", "suffrage": "voter"},
#     {"id": "conure-0", "address": "10.42.0.117:7001", "suffrage": "voter"},
#     {"id": "conure-1", "address": "10.42.0.118:7001", "suffrage": "voter"}
#   ]
# }
```

#### Get Raft Statistics
```bash
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl "http://localhost:8081/raft/stats"

# Response: Detailed raft metrics including log indices, terms, etc.
```

#### Add/Remove Cluster Members (Advanced)
```bash
# Add a new voter (usually handled automatically)
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl -X POST "http://localhost:8081/join" \
  -H 'Content-Type: application/json' \
  -d '{"ID":"new-node","RaftAddr":"10.42.0.119:7001"}'

# Remove a node (usually handled by preStop hooks)
kubectl exec conure-bootstrap-0 -n <namespace> -- \
  curl -X POST "http://localhost:8081/remove" \
  -H 'Content-Type: application/json' \
  -d '{"ID":"old-node"}'
```

### External Access

#### Port Forwarding for Development
```bash
# Forward HTTP API port
kubectl port-forward conure-bootstrap-0 8081:8081 -n <namespace>

# Now you can access the API locally
curl "http://localhost:8081/status"
curl -X PUT "http://localhost:8081/kv?key=test&value=hello"
curl "http://localhost:8081/kv?key=test"
```

#### Service Exposure
```bash
# Expose via LoadBalancer (cloud environments)
kubectl patch service conure -n <namespace> -p '{"spec":{"type":"LoadBalancer"}}'

# Expose via NodePort
kubectl patch service conure -n <namespace> -p '{"spec":{"type":"NodePort"}}'

# Expose via Ingress
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: conuredb-api
  namespace: <namespace>
spec:
  rules:
  - host: conuredb.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: conure
            port:
              number: 8081
EOF
```

### API Error Handling

```bash
# Non-existent key returns empty response
curl "http://localhost:8081/kv?key=nonexistent"
# Response: (empty)

# Missing key parameter
curl -X PUT "http://localhost:8081/kv?value=test"
# Response: missing key

# Leader redirect (if querying a follower)
curl "http://follower-node:8081/join" -d '{"ID":"test","RaftAddr":"test"}'
# Response: HTTP 409 with leader hint
# {"leader":"10.42.0.116:7001"}
```

### Production Usage Examples

#### Health Check Script
```bash
#!/bin/bash
NAMESPACE="production"
POD="conure-bootstrap-0"

# Check if cluster has a leader
LEADER=$(kubectl exec $POD -n $NAMESPACE -- curl -s http://localhost:8081/status | grep -o '"leader":"[^"]*"' | cut -d'"' -f4)

if [ -n "$LEADER" ] && [ "$LEADER" != "" ]; then
    echo "✅ Cluster healthy - Leader: $LEADER"
    exit 0
else
    echo "❌ Cluster unhealthy - No leader found"
    exit 1
fi
```

#### Backup Script
```bash
#!/bin/bash
NAMESPACE="production"
BACKUP_DIR="/backups/$(date +%Y%m%d_%H%M%S)"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Export cluster configuration
kubectl exec conure-bootstrap-0 -n $NAMESPACE -- \
  curl -s http://localhost:8081/raft/config > "$BACKUP_DIR/cluster.json"

# Copy persistent volume data (platform-specific)
kubectl exec conure-bootstrap-0 -n $NAMESPACE -- \
  tar czf - /var/lib/conure > "$BACKUP_DIR/data.tar.gz"

echo "Backup completed: $BACKUP_DIR"
```

## Verification

After deployment, verify cluster status:

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=conuredb

# Check cluster configuration
kubectl exec conure-0 -- curl -s http://localhost:8081/raft/config

# Test key-value operations
kubectl exec conure-0 -- curl -X PUT "http://localhost:8081/kv?key=test" -d "value"
kubectl exec conure-0 -- curl "http://localhost:8081/kv?key=test"
```

## Troubleshooting

### Common Issues

1. **Pods stuck in Init state**: Check if bootstrap node is ready
2. **Raft join failures**: Verify seed configuration and network connectivity
3. **Storage issues**: Check PVC creation and storage class

### Debug Commands

```bash
# View logs
kubectl logs -f conure-0
kubectl logs -f conure-arbiter-0

# Check raft status
kubectl exec conure-0 -- curl -s http://localhost:8081/raft/stats

# Check cluster members
kubectl exec conure-0 -- curl -s http://localhost:8081/raft/config
```

## Scaling

### Automated Scaling

The chart now supports fully automated scaling with zero manual intervention:

```bash
# Scale up: 1 → 3 nodes (adds voter nodes)
helm upgrade conuredb ./charts/conuredb --set voters.replicas=3

# Scale down: 3 → 1 node (automatically removes nodes from raft first)
helm upgrade conuredb ./charts/conuredb --set voters.replicas=1

# Scale to any size
helm upgrade conuredb ./charts/conuredb --set voters.replicas=5
```

### How Automated Scaling Works

1. **Scale Up**: New voter pods join the existing raft cluster automatically
2. **Scale Down**:
   - Helm pre-upgrade hook removes excess nodes from raft cluster
   - StatefulSet scaling reduces pod count
   - PreStop hooks handle graceful node removal if needed
3. **Arbiter Management**: Automatically adds/removes arbiter based on voter count

### Scaling Configuration

```yaml
# In values.yaml
scaling:
  autoRemoveNodes: true  # Enable automated node removal (default: true)

voters:
  replicas: 3  # Target number of voter nodes (including bootstrap)
```

### Manual Scaling (Legacy)

If you disable automated scaling (`scaling.autoRemoveNodes: false`), you'll need to manually remove nodes:

```bash
# Before scaling down, remove nodes from raft cluster
kubectl exec conure-bootstrap-0 -n <namespace> -- curl -X POST http://localhost:8081/remove -H 'Content-Type: application/json' -d '{"ID":"conure-2"}'

# Then scale down
helm upgrade conuredb ./charts/conuredb --set voters.replicas=2
```

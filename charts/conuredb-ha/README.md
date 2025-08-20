# ConureDB High Availability

A production-ready high-availability ConureDB deployment with raft consensus.

## Features

- **High Availability**: Minimum 3 nodes required for raft quorum
- **Scaling Protection**: Cannot scale below 3 replicas
- **Raft Consensus**: Distributed consensus with leader election
- **Bootstrap Node**: Stable bootstrap node for cluster initialization
- **Graceful Scaling**: Proper node removal during scale-down operations
- **Pod Disruption Budget**: Protects cluster availability during maintenance

## Installation

```bash
# Install HA ConureDB (minimum 3 nodes)
helm install my-conuredb-ha ./charts/conuredb-ha

# Scale up (can scale to any number >= 3)
helm upgrade my-conuredb-ha ./charts/conuredb-ha --set voters.replicas=5

# Access the service
kubectl port-forward svc/conure-client 8081:8081
curl http://localhost:8081/status
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | ConureDB image repository | `docker.io/conuredb/conuredb` |
| `image.tag` | ConureDB image tag | `latest-2` |
| `voters.replicas` | Number of voter nodes (min 3) | `3` |
| `voters.resources.requests.cpu` | CPU request per voter | `50m` |
| `voters.resources.requests.memory` | Memory request per voter | `128Mi` |
| `voters.pvc.size` | Storage size per voter | `10Gi` |
| `bootstrap.preventDeletion` | Prevent bootstrap deletion on uninstall | `false` |
| `pdb.enabled` | Enable Pod Disruption Budget | `true` |

## Architecture

- **Bootstrap Node**: `conure-bootstrap-0` - stable anchor for the cluster
- **Voter Nodes**: `conure-0`, `conure-1`, ... - additional voting members
- **Services**:
  - `conure-client` - load-balanced client access
  - `conure-hs` - headless service for internal communication

## Scaling Operations

### Scale Up

```bash
# Scale to 5 nodes
helm upgrade my-conuredb-ha ./charts/conuredb-ha --set voters.replicas=5
```

### Scale Down

```bash
# Scale to 3 nodes (minimum)
helm upgrade my-conuredb-ha ./charts/conuredb-ha --set voters.replicas=3
```

**Note**: The chart includes pre-upgrade hooks that safely remove nodes from the raft cluster before scaling down.

## Use Cases

- Production environments
- Applications requiring high availability
- Multi-region deployments
- Critical data storage

## Requirements

- **Minimum Nodes**: 3 replicas required
- **Storage**: Persistent volumes for each node
- **Network**: Stable network connectivity between nodes

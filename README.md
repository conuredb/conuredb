# ConureDB

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/conure-db/conure-db)](https://goreportcard.com/report/github.com/conure-db/conure-db)
[![Docker Pulls](https://img.shields.io/docker/pulls/conuredb/conuredb)](https://hub.docker.com/r/conuredb/conuredb)

A B-Tree based key-value store with copy-on-write pages and a Raft-backed distributed mode for linearizable writes and optional follower reads.

## 🎯 Features

- **Raft Consensus**: Linearizable writes with HashiCorp Raft implementation
- **B-Tree Storage**: Efficient key-value storage with copy-on-write pages
- **HTTP API**: RESTful interface for all operations
- **Kubernetes Native**: Production-ready Helm charts with automated scaling
- **High Availability**: Smart quorum management with optional arbiter nodes
- **Consistent Reads**: Linearizable leader reads, optional stale follower reads
- **Zero-Downtime Scaling**: Automated node addition/removal during scaling operations

## 🔧 Consistency Model

- **Writes**: Linearizable via Raft (acknowledged after commit on quorum)
- **Reads**:
  - **Leader reads**: Linearizable (API issues a Raft barrier)
  - **Follower reads**: Eventually consistent with `stale=true` parameter
- YAML config + CLI overrides
- Remote-only REPL that talks to the HTTP API and follows leader redirects

## Consistency model

- Writes: linearizable via Raft (ack after commit on quorum)
- Reads:
  - Leader reads are linearizable (API issues a Raft barrier)
  - Follower reads with `stale=true` may lag (eventually consistent)

## 📦 Installation

### Quick Start with Docker

```bash
# Run single node
docker run -d --name conuredb \
  -p 8081:8081 \
  conuredb/conuredb:latest \
  --node-id=node1 --bootstrap

# Test the API
curl -X PUT "http://localhost:8081/kv?key=hello&value=world"
curl "http://localhost:8081/kv?key=hello"
```

### Kubernetes Deployment

ConureDB provides production-ready Helm charts for Kubernetes deployment:

```bash
# Add the Helm repository (when available)
helm repo add conuredb https://charts.conuredb.dev
helm repo update

# Install single-node for development
helm install conuredb conuredb/conuredb-single

# Install HA cluster for production (minimum 3 nodes)
helm install conuredb conuredb/conuredb-ha \
  --set voters.replicas=3 \
  --set voters.pvc.size=20Gi
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/conure-db/conure-db.git
cd conure-db

# Build binaries
go build ./cmd/conure-db
go build ./cmd/repl

# Run tests
go test ./...
```

## 🏗️ Architecture

ConureDB uses a multi-component architecture optimized for Kubernetes:

### Local Development

- **Single Process**: One binary handles both bootstrap and storage
- **File-based Storage**: Local B-Tree files with Raft logs

### Kubernetes Production

- **Bootstrap Node**: Dedicated StatefulSet for cluster initialization
- **Voter Nodes**: Scalable StatefulSet for data storage and voting
- **Arbiter Nodes**: Optional tie-breaker nodes for even-numbered clusters
- **Smart Scaling**: Automated Raft membership management during scaling

### Quorum Logic

- **Odd clusters** (3, 5, 7 nodes): Natural majority, no arbiter needed
- **Even clusters** (2, 4, 6 nodes): Arbiter node added automatically for quorum
- **Arbiter modes**:
  - `auto` (default): Arbiter added only for even-numbered clusters
  - `always`: Always deploy an arbiter
  - `never`: Never deploy an arbiter

## ⚙️ Configuration

ConureDB supports both YAML configuration files and command-line flags.

### Configuration File

Create a `config.yaml` file:

```yaml
node_id: node1
data_dir: ./data/node1
raft_addr: 127.0.0.1:7001
http_addr: :8081
bootstrap: true
barrier_timeout: 3s
```

### Command Line Flags

Flags override YAML configuration:

- `--config` string: Path to YAML configuration file
- `--node-id` string: Unique node identifier (stable across restarts)
- `--data-dir` string: Directory for database and Raft state
- `--raft-addr` string: Raft bind/advertise address (host:port)
- `--http-addr` string: HTTP API bind address
- `--bootstrap`: Bootstrap single-node cluster if no existing state
- `--barrier-timeout` duration: Leader read barrier timeout (e.g., `3s`)

### Defaults

If not specified anywhere:

- `node_id=node1`
- `data_dir=./data`
- `raft_addr=127.0.0.1:7001`
- `http_addr=:8081`
- `bootstrap=true`
- `barrier_timeout=3s`

## 🚀 Usage Examples

### Single Node (Development)

```bash
# Start a single-node cluster
./conure-db --node-id=node1 --data-dir=./data/node1 --bootstrap

# Put a key-value pair
curl -X PUT 'http://localhost:8081/kv?key=hello&value=world'

# Get the value
curl 'http://localhost:8081/kv?key=hello'

# Check cluster status
curl 'http://localhost:8081/status'
```

### Multi-Node Cluster (Local)

```bash
# Terminal 1: Start bootstrap node
./conure-db --node-id=node1 --data-dir=./data/node1 \
  --raft-addr=127.0.0.1:7001 --http-addr=:8081 --bootstrap

# Terminal 2: Start second node
./conure-db --node-id=node2 --data-dir=./data/node2 \
  --raft-addr=127.0.0.1:7002 --http-addr=:8082

# Terminal 3: Start third node
./conure-db --node-id=node3 --data-dir=./data/node3 \
  --raft-addr=127.0.0.1:7003 --http-addr=:8083

# Join nodes to cluster (from any terminal)
curl -X POST 'http://localhost:8081/join' \
  -H 'Content-Type: application/json' \
  -d '{"ID":"node2","RaftAddr":"127.0.0.1:7002"}'

curl -X POST 'http://localhost:8081/join' \
  -H 'Content-Type: application/json' \
  -d '{"ID":"node3","RaftAddr":"127.0.0.1:7003"}'

# Verify cluster configuration
curl 'http://localhost:8081/raft/config'
```

## 🔌 HTTP API Reference

### Key-Value Operations

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| `PUT` | `/kv?key=<key>&value=<value>` | Store key-value pair | `PUT /kv?key=user&value=alice` |
| `PUT` | `/kv?key=<key>` (body) | Store with request body | `PUT /kv?key=config` + JSON body |
| `GET` | `/kv?key=<key>` | Get value (linearizable) | `GET /kv?key=user` |
| `GET` | `/kv?key=<key>&stale=true` | Get value (eventually consistent) | `GET /kv?key=user&stale=true` |
| `DELETE` | `/kv?key=<key>` | Delete key | `DELETE /kv?key=user` |

### Cluster Management

| Method | Endpoint | Description | Response |
|--------|----------|-------------|----------|
| `GET` | `/status` | Get node and leader status | `{"is_leader":true,"leader":"..."}` |
| `GET` | `/raft/config` | Get cluster membership | List of nodes with IDs and addresses |
| `GET` | `/raft/stats` | Get Raft statistics | Detailed Raft metrics |
| `POST` | `/join` | Add node to cluster | `{"ID":"node2","RaftAddr":"..."}` |
| `POST` | `/remove` | Remove node from cluster | `{"ID":"node2"}` |

### Examples

```bash
# Store data
curl -X PUT "http://localhost:8081/kv?key=app&value=conuredb"
echo '{"database":"prod","version":"1.0"}' | \
  curl -X PUT "http://localhost:8081/kv?key=config" -d @-

# Read data
curl "http://localhost:8081/kv?key=app"
curl "http://localhost:8081/kv?key=config&stale=true"  # Allow stale reads

# Cluster operations
curl "http://localhost:8081/status"
curl "http://localhost:8081/raft/config"
```

## 🎮 Interactive REPL

ConureDB includes a remote REPL that connects to the HTTP API:

```bash
# Connect to default server (localhost:8081)
./repl

# Connect to specific server
./repl --server=http://127.0.0.1:8081
```

### REPL Commands

- `put <key> <value>` - Store a key-value pair
- `get <key>` - Retrieve a value
- `delete <key>` - Delete a key
- `help` - Show available commands
- `exit` - Exit the REPL

The REPL automatically follows leader redirects and handles cluster topology changes.

## ☸️ Kubernetes Deployment

### Helm Charts

ConureDB provides two Helm charts optimized for different use cases:

#### Single Node Chart (`conuredb-single`)

- **Use case**: Development, testing, demo environments
- **Features**: Single replica, immediate bootstrap, minimal resources
- **Scaling**: Prevents scaling beyond 1 replica

```bash
# Install single-node chart
helm install mydb ./charts/conuredb-single

# Customization
helm install mydb ./charts/conuredb-single \
  --set image.tag=v1.0.0 \
  --set pvc.size=5Gi
```

#### High Availability Chart (`conuredb-ha`)

- **Use case**: Production environments requiring high availability
- **Features**: Multi-node cluster, automated scaling, arbiter support
- **Scaling**: Minimum 3 nodes, automated Raft membership management

```bash
# Install HA cluster (3 nodes minimum)
helm install mydb ./charts/conuredb-ha \
  --set voters.replicas=3

# Production configuration
helm install mydb ./charts/conuredb-ha \
  --set voters.replicas=5 \
  --set voters.pvc.size=100Gi \
  --set voters.pvc.storageClassName=fast-ssd \
  --set voters.resources.requests.cpu=500m \
  --set voters.resources.requests.memory=1Gi
```

### Automated Scaling

The HA chart supports zero-downtime scaling:

```bash
# Scale up (adds nodes to Raft cluster automatically)
helm upgrade mydb ./charts/conuredb-ha --set voters.replicas=5

# Scale down (removes nodes from Raft cluster first)
helm upgrade mydb ./charts/conuredb-ha --set voters.replicas=3
```

### Accessing the API in Kubernetes

```bash
# Port forward for local access
kubectl port-forward svc/conure 8081:8081

# Direct pod access
kubectl exec -it conure-bootstrap-0 -- \
  curl "http://localhost:8081/status"

# Service endpoints
kubectl get endpoints conure
```

### Configuration Examples

#### Development Cluster

```yaml
# values-dev.yaml
voters:
  replicas: 3
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
  pvc:
    size: 1Gi

arbiter:
  mode: never
```

#### Production Cluster

```yaml
# values-prod.yaml
voters:
  replicas: 5
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 1000m
      memory: 2Gi
  pvc:
    size: 100Gi
    storageClassName: fast-ssd

security:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000

pdb:
  enabled: true

nodeSelector:
  node-type: database

tolerations:
  - key: database
    operator: Equal
    value: "true"
    effect: NoSchedule
```

### Monitoring and Observability

```bash
# Check cluster health
kubectl exec conure-bootstrap-0 -- \
  curl -s "http://localhost:8081/raft/config" | jq '.'

# View Raft statistics
kubectl exec conure-bootstrap-0 -- \
  curl -s "http://localhost:8081/raft/stats" | jq '.'

# Check pod status
kubectl get pods -l app.kubernetes.io/name=conuredb

# View logs
kubectl logs -f conure-bootstrap-0
kubectl logs -l app.kubernetes.io/name=conuredb --tail=100
```

## 🐛 Troubleshooting

### Common Issues and Solutions

#### No Leader Elected

**Symptoms**: `/status` shows empty leader, cluster appears stuck

**Causes & Solutions**:

- **Bootstrap not applied**: Ensure clean `data_dir` on first start with `--bootstrap`
- **Lost quorum**: Maintain odd number of voters (3, 5, 7) for natural majority
- **Network partition**: Check connectivity between nodes

```bash
# Check cluster configuration
curl "http://localhost:8081/raft/config"

# Verify all nodes are reachable
curl "http://localhost:8081/status"
curl "http://localhost:8082/status"
```

#### Stale Read Issues

**Symptoms**: Follower reads with `stale=true` return "key not found"

**Explanation**: Expected behavior briefly after leader writes; followers will catch up

**Solution**: Use leader reads for guaranteed consistency:

```bash
# Guaranteed consistent read (from leader)
curl "http://localhost:8081/kv?key=mykey"

# Eventually consistent read (from follower)
curl "http://localhost:8082/kv?key=mykey&stale=true"
```

#### Heartbeat Errors to Removed Peers

**Symptoms**: Logs show heartbeat failures to nodes that should be removed

**Causes & Solutions**:

- **Removal not committed**: Ensure removal was issued to the correct leader
- **Wrong node ID**: Verify node IDs match exactly in `/raft/config`
- **Quorum lost during removal**: Membership changes require active quorum

```bash
# Verify cluster membership before removal
curl "http://localhost:8081/raft/config"

# Remove node properly
curl -X POST "http://localhost:8081/remove" 
  -H 'Content-Type: application/json' 
  -d '{"ID":"exact-node-id-from-config"}'
```

#### Data Directory Conflicts

**Symptoms**: Multiple database files, startup errors

**Solution**: Each node needs unique `--data-dir`:

```bash
# Correct: unique directories
./conure-db --node-id=node1 --data-dir=./data/node1
./conure-db --node-id=node2 --data-dir=./data/node2

# Incorrect: shared directory
./conure-db --node-id=node1 --data-dir=./data  # ❌
./conure-db --node-id=node2 --data-dir=./data  # ❌
```

### Kubernetes Troubleshooting

#### Pods Stuck in Init State

```bash
# Check if bootstrap node is ready
kubectl get pods -l app.kubernetes.io/name=conuredb
kubectl logs conure-bootstrap-0

# Check init container logs
kubectl logs conure-0 -c wait-for-bootstrap
```

#### Scaling Issues

```bash
# Check pre-scale job logs
kubectl logs job/conure-pre-scale-<revision>

# Verify current cluster state
kubectl exec conure-bootstrap-0 -- 
  curl -s "http://localhost:8081/raft/config"

# Check StatefulSet status
kubectl describe statefulset conure
```

#### Storage Issues

```bash
# Check PVC status
kubectl get pvc

# Verify storage class exists
kubectl get storageclass

# Check pod events
kubectl describe pod conure-0
```

### Debug Commands

```bash
# Local debugging
curl "http://localhost:8081/raft/stats"    # Detailed Raft metrics
curl "http://localhost:8081/raft/config"   # Current cluster membership
curl "http://localhost:8081/status"        # Leader status

# Kubernetes debugging
kubectl exec conure-bootstrap-0 -- curl -s "http://localhost:8081/raft/stats"
kubectl logs -f conure-bootstrap-0
kubectl describe pod conure-bootstrap-0
```

## 🔧 Development

### Prerequisites

- Go 1.23.0 or later
- Docker (for containerized testing)
- Kubernetes cluster (for Helm chart testing)

### Building

```bash
# Install dependencies
go mod download

# Build all binaries
go build ./...

# Build specific components
go build -o bin/conure-db ./cmd/conure-db
go build -o bin/repl ./cmd/repl

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o bin/conure-db-linux ./cmd/conure-db
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./pkg/api
go test ./btree

# Benchmark tests
go test -bench=. ./btree
```

### Docker Development

```bash
# Build Docker image
docker build -t conuredb/conuredb:dev .

# Run containerized tests
docker run --rm conuredb/conuredb:dev go test ./...

# Multi-stage build for minimal image
docker build --target=production -t conuredb/conuredb:latest .
```

### Helm Chart Development

```bash
# Lint charts
helm lint charts/conuredb-single
helm lint charts/conuredb-ha

# Template generation (dry-run)
helm template test charts/conuredb-ha --debug

# Install in development namespace
helm install dev charts/conuredb-ha 
  --namespace dev 
  --create-namespace 
  --set image.tag=dev

# Test scaling
helm upgrade dev charts/conuredb-ha --set voters.replicas=5
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Quick Start for Contributors

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Ensure all tests pass: `go test ./...`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to your branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Areas for Contribution

- **Performance optimization**: B-Tree improvements, Raft tuning
- **Security features**: Authentication, encryption at rest
- **Monitoring**: Metrics, observability improvements
- **Documentation**: API examples, deployment guides
- **Testing**: Integration tests, chaos engineering
- **Client libraries**: SDK for various programming languages

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Documentation**: [GitHub Wiki](https://github.com/conure-db/conure-db/wiki)
- **Docker Images**: [Docker Hub](https://hub.docker.com/r/conuredb/conuredb)
- **Issues**: [GitHub Issues](https://github.com/conure-db/conure-db/issues)
- **Discussions**: [GitHub Discussions](https://github.com/conure-db/conure-db/discussions)
- **Security**: [Security Policy](SECURITY.md)

## 🎯 Roadmap

- [ ] **Authentication & Authorization**: Built-in user management
- [ ] **Encryption at Rest**: Transparent data encryption
- [ ] **Backup & Restore**: Automated backup solutions
- [ ] **Metrics & Monitoring**: Prometheus integration
- [ ] **Client SDKs**: Libraries for popular languages
- [ ] **Query Language**: SQL-like interface for complex queries
- [ ] **Streaming**: Change data capture and event streaming
- [ ] **Multi-Region**: Cross-datacenter replication

---

**ConureDB** - Building distributed systems, one key at a time. 🦜

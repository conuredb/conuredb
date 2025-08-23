# ConureDB Single Node

A simplified single-node ConureDB deployment for development and testing.

## Features

- **Single Node**: Always runs exactly 1 replica
- **No Scaling**: Replicas are locked to 1 - cannot scale up
- **Simple Setup**: No complex raft configuration or joining logic
- **Bootstrap Mode**: Runs in bootstrap mode for immediate availability
- **No PDB**: Pod Disruption Budget disabled (not needed for single node)

## Installation

```bash
# Install single-node ConureDB
helm install my-conuredb ./charts/conuredb-single

# Access the service
kubectl port-forward svc/conure-client 8081:8081
curl http://localhost:8081/status
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | ConureDB image repository | `docker.io/conuredb/conuredb` |
| `image.tag` | ConureDB image tag | `latest-2` |
| `single.resources.requests.cpu` | CPU request | `100m` |
| `single.resources.requests.memory` | Memory request | `256Mi` |
| `single.pvc.size` | Storage size | `10Gi` |

## Use Cases

- Development environments
- Testing and CI/CD
- Small applications that don't need HA
- Learning and experimentation

## Limitations

- **No High Availability**: Single point of failure
- **No Scaling**: Cannot scale beyond 1 replica
- **Data Loss Risk**: If the pod dies, data is only preserved if using persistent storage

# ConureDB

A B-Tree based key-value store with copy-on-write pages and a Raft-backed distributed mode for linearizable writes and optional follower reads.

## Highlights

- Raft replication (HashiCorp Raft): quorum-committed writes, leader election, snapshots/restore
- HTTP API: GET/PUT/DELETE, join/remove, status, raft config and stats
- Linearizable reads from leader; optional follower reads with `stale=true`
- YAML config + CLI overrides
- Remote-only REPL that talks to the HTTP API and follows leader redirects

## Consistency model

- Writes: linearizable via Raft (ack after commit on quorum)
- Reads:
  - Leader reads are linearizable (API issues a Raft barrier)
  - Follower reads with `stale=true` may lag (eventually consistent)

## Processes / Binaries

- `cmd/conure-db`: Raft-backed server
- `cmd/repl`: Remote-only REPL (uses the HTTP API)

## Configuration

You can run purely with flags or provide a YAML file and optionally override via flags.

Supported YAML (file path via `--config`):

```yaml
  node_id: node1
  data_dir: ./data/node1
  raft_addr: 127.0.0.1:7001
  http_addr: :8081
  bootstrap: true
  barrier_timeout: 3s
```

Flags (override YAML when provided):

- `--config` string: path to YAML
- `--node-id` string: node identity (stable)
- `--data-dir` string: directory for node state (DB + raft)
- `--raft-addr` string: Raft bind/advertise address (host:port)
- `--http-addr` string: HTTP bind address (default from config)
- `--bootstrap`: bootstrap single-node cluster if no existing state
- `--barrier-timeout` duration: leader read barrier timeout (e.g., `3s`)

Defaults if not set anywhere:

- `node_id=node1`, `data_dir=./data`, `raft_addr=127.0.0.1:7001`, `http_addr=:8081`, `bootstrap=true`, `barrier_timeout=3s`

## Run: single node (local)

```bash
# Start leader (single node)
go run ./cmd/conure-db --node-id=node1 --data-dir=./data/node1 --raft-addr=127.0.0.1:7001 --http-addr=:8081 --bootstrap

# Put a key
echo -n 'v1' | curl -sS -X PUT 'http://localhost:8081/kv?key=k1' --data-binary @-
# Get the key
curl -sS 'http://localhost:8081/kv?key=k1'
# Status
curl -sS 'http://localhost:8081/status'
```

## Run: 3-node cluster (local machine)

```bash
# node1 (bootstrap)
go run ./cmd/conure-db --node-id=node1 --data-dir=./data/node1 --raft-addr=127.0.0.1:7001 --http-addr=:8081 --bootstrap
# node2
go run ./cmd/conure-db --node-id=node2 --data-dir=./data/node2 --raft-addr=127.0.0.1:7002 --http-addr=:8082
# node3
go run ./cmd/conure-db --node-id=node3 --data-dir=./data/node3 --raft-addr=127.0.0.1:7003 --http-addr=:8083

# Join node2 and node3 via the current leader (assume :8081)
curl -sS -X POST http://localhost:8081/join -H 'Content-Type: application/json' -d '{"ID":"node2","RaftAddr":"127.0.0.1:7002"}'
curl -sS -X POST http://localhost:8081/join -H 'Content-Type: application/json' -d '{"ID":"node3","RaftAddr":"127.0.0.1:7003"}'

# Inspect cluster config
curl -sS http://localhost:8081/raft/config
```

### Removing a node

Membership changes require quorum. Remove from the leader before shutting down a node:

```bash
curl -sS -X POST http://localhost:8081/remove -H 'Content-Type: application/json' -d '{"ID":"node3"}'
```

Verify with `/raft/config` afterwards. If you scale down without removal, the leader will keep trying to heartbeat to the missing node.

## HTTP API

- `PUT /kv?key=<k>` body=value → leader-only, replicated write
- `GET /kv?key=<k>` → leader-only linearizable read (followers redirect with leader hint)
- `GET /kv?key=<k>&stale=true` → allow follower to serve a potentially stale read
- `DELETE /kv?key=<k>` → leader-only, replicated delete
- `GET /status` → `{ is_leader, leader }`
- `POST /join` body=`{"ID":"node2","RaftAddr":"127.0.0.1:7002"}` → add voter (leader-only)
- `POST /remove` body=`{"ID":"node2"}` → remove server (leader-only)
- `GET /raft/config` → cluster membership (ID, address, suffrage)
- `GET /raft/stats` → Raft runtime stats (term, indices, last_contact, state)

## REPL (remote-only)

The REPL always uses the HTTP API, keeping data in sync and replicated by Raft.

```bash
# default server on :8081
go run ./cmd/repl
# specify server explicitly
go run ./cmd/repl --server=http://127.0.0.1:8081
```

Commands:

- `put <key> <value>`
- `get <key>`
- `delete <key>`
- `help`, `exit`

Notes:

- REPL writes go to the leader (the client follows leader redirects automatically).
- For follower reads with stale tolerance, use the HTTP API from your app with `stale=true`.

## Troubleshooting

- Leader unknown (`/status` shows leader empty): bootstrap wasn’t applied or lost quorum. Ensure clean `data_dir` on first bootstrap; keep an odd number of voters (3/5).
- Follower `stale=true` read shows "key not found": expected briefly after leader writes; follower will catch up. For guaranteed visibility, read from leader.
- Heartbeat errors to removed peer: removal may not have committed or was issued against the wrong leader/ID; verify `/raft/config`. Membership changes need quorum.
- Multiple DB files under `./data`: each node must use a unique `--data-dir` (e.g., `./data/node1`). REPL uses the API and does not access files directly.

## Develop

```bash
# build all
go build ./...
# run tests
go test ./...
```

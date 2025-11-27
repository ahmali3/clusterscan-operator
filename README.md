# 🛡️ ClusterScan Operator

A Kubernetes operator for scheduling security scans with first-class CronJob support.

[![Go](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.28+-326CE5.svg)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)

---

## Features

| Feature | Description |
|---------|-------------|
| **One-off & Scheduled Execution** | Creates Jobs for immediate scans or CronJobs for recurring schedules |
| **Webhook Validation** | Rejects invalid cron syntax at admission time |
| **Smart Defaults** | Auto-fills Trivy commands when only image is specified |
| **Status Tracking** | Kubernetes-native Phase, Conditions, and LastRunTime |
| **Automatic Cleanup** | Owner references ensure Jobs/CronJobs are garbage collected |

## Architecture

```
┌─────────────────┐
│  ClusterScan CR │
└────────┬────────┘
         │
         ▼
   ┌─────────────┐
   │  Webhooks   │ ◄── Validate cron, mutate defaults
   └──────┬──────┘
          │
          ▼
   ┌──────────────┐
   │  Controller  │
   └──────┬───────┘
          │
    ┌─────┴─────┐
    ▼           ▼
┌──────┐   ┌─────────┐
│ Job  │   │ CronJob │
└──────┘   └─────────┘
```

**How it works:**
- **No schedule specified** → Controller creates a Job (one-off execution)
- **Schedule specified** → Controller creates a CronJob (recurring execution)
- **Webhooks** → Mutate defaults and validate before creation

---

## Quick Start

### 1. Setup Cluster & Cert-Manager

```bash
# Create cluster
kind create cluster --name demo

# Install cert-manager (required for webhook TLS)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=180s
```

### 2. Deploy Operator

```bash
# Install CRDs
make install

# Build and load image
make docker-build IMG=clusterscan:v1
kind load docker-image clusterscan:v1 --name demo

# Deploy
make deploy IMG=clusterscan:v1

# Verify
kubectl get pods -n clusterscan-operator-system
```

### 3. Run Your First Scan

```bash
kubectl apply -f samples/demo-default.yaml
kubectl get clusterscans
kubectl describe clusterscan demo-default
```

---

## Examples

### One-off Scan

```yaml
apiVersion: scan.ahmali3.github.io/v1alpha1
kind: ClusterScan
metadata:
  name: quick-scan
spec:
  image: aquasec/trivy:latest
  command: ["trivy", "image", "nginx:latest"]
```

```bash
kubectl apply -f quick-scan.yaml
kubectl logs job/quick-scan-job
```

### Scheduled Scan

```yaml
apiVersion: scan.ahmali3.github.io/v1alpha1
kind: ClusterScan
metadata:
  name: nightly-audit
spec:
  image: aquasec/trivy:latest
  schedule: "0 2 * * *"  # 2 AM daily
  # command auto-filled by mutating webhook
```

```bash
kubectl apply -f nightly-audit.yaml
kubectl get cronjobs
```

### Suspend Scheduling

```yaml
spec:
  schedule: "0 2 * * *"
  suspend: true  # Temporarily pause
```

---

## Live Demos

All samples available in [`samples/`](samples/) directory.

**Demo 1: Webhook Mutation** (Auto-fills Trivy command)
```bash
kubectl apply -f samples/demo-default.yaml
kubectl get clusterscan demo-default -o yaml
# Shows: command: ["trivy", "image", "nginx:1.19"]
```

**Demo 2: Webhook Validation** (Rejects bad cron syntax)
```bash
kubectl apply -f samples/demo-fail.yaml
# Error: admission webhook denied the request: Invalid cron schedule
```

**Demo 3: Production Scans** (Nightly Trivy + Weekly kube-bench)
```bash
kubectl apply -f samples/real-world-scans.yaml
kubectl get clusterscans
```

---

## Observability

```bash
# View status
kubectl get clusterscans
kubectl describe clusterscan <name>

# Check logs
kubectl logs job/<name>-job

# Watch events
kubectl get events -w
```

**Status fields:**
```yaml
status:
  phase: Completed  # Pending, Running, Completed, Scheduled, Failed
  lastRunTime: "2024-11-26T20:15:30Z"
  lastJobName: "demo-default-job"
  conditions:
    - type: Ready
      status: "True"
      reason: Completed
```

---

## Development

```bash
# Run operator locally
make install
make run

# Run tests
make test

# Update dependencies
go mod tidy
```

---

## Project Structure

```
clusterscan-operator/
├── api/v1alpha1/              # CRD definitions
├── internal/
│   ├── controller/            # Reconciliation logic
│   └── webhook/v1alpha1/      # Admission webhooks
├── samples/                   # Demo manifests
├── config/                    # Kustomize configs
└── Dockerfile                 # Multi-stage build
```

---

## License

Apache 2.0

---

Built with [Kubebuilder](https://book.kubebuilder.io/) • Powered by [Trivy](https://trivy.dev/)

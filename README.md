# 🛡️ ClusterScan Operator

A Kubernetes operator for automated security scanning with support for one-time and scheduled scans.

[![Go](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.28+-326CE5.svg)](https://kubernetes.io)

---

## ✨ Features

- **One-time & Scheduled Scans** - Run scans immediately or on a cron schedule
- **Multiple Scanners** - Trivy, Kube-bench, Grype, and custom scanners
- **Webhook Validation** - Automatic validation of configurations
- **Smart Defaults** - Auto-fills scanner commands for common tools
- **Result Export** - Save scan results locally with timestamps
- **Suspend/Resume** - Dynamic control over scheduled scans

---

## 🚀 Quick Start

### Prerequisites

- [Kind](https://kind.sigs.k8s.io/) or any Kubernetes cluster
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Docker](https://www.docker.com/)
- [Go 1.24+](https://golang.org/)

### Automated Setup (Recommended)

```bash
git clone https://github.com/ahmali3/clusterscan-operator.git
cd clusterscan-operator

# Setup cluster, cert-manager, and operator
make setup

# Run interactive demo
make demo
```

### Manual Setup

```bash
# Create cluster
kind create cluster --name clusterscan

# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=available --timeout=120s deployment/cert-manager -n cert-manager
kubectl wait --for=condition=available --timeout=120s deployment/cert-manager-webhook -n cert-manager

# Install CRDs
make install

# Build and load image
make docker-build IMG=controller:latest
kind load docker-image controller:latest --name clusterscan

# Deploy operator
make deploy IMG=controller:latest

# Wait for operator
kubectl wait --for=condition=available --timeout=120s \
  deployment/clusterscan-operator-controller-manager -n clusterscan-operator-system
```

---

## 📖 Usage

### Simple Scan

```yaml
apiVersion: scan.ahmali3.github.io/v1alpha1
kind: ClusterScan
metadata:
  name: nginx-scan
spec:
  target: nginx:latest
```

```bash
kubectl apply -f nginx-scan.yaml
kubectl get clusterscans -w
```

### Scheduled Scan

```yaml
apiVersion: scan.ahmali3.github.io/v1alpha1
kind: ClusterScan
metadata:
  name: nightly-scan
spec:
  target: python:3.9
  schedule: "0 2 * * *"
```

### Custom Scanner

```yaml
apiVersion: scan.ahmali3.github.io/v1alpha1
kind: ClusterScan
metadata:
  name: kube-bench
spec:
  image: aquasec/kube-bench:latest
  command: ["kube-bench", "run", "--targets", "master,node"]
```

### Suspend Schedule

```bash
kubectl patch clusterscan nightly-scan --type=merge -p '{"spec":{"suspend":true}}'
```

---

## 🎬 Interactive Demo

```bash
# Run demo with menu
make demo

# Or apply samples manually
kubectl apply -f samples/00-simple-scan.yaml
kubectl apply -f samples/03-scheduled.yaml
kubectl apply -f samples/04-vulnerability-scan.yaml
```

---

## 📊 View Results

### Check Status

```bash
# List all scans
make show-scans

# Or use kubectl
kubectl get clusterscans
kubectl describe clusterscan <name>
```

### Export Results

```bash
# Using scripts
make export-results SCAN=nginx-scan
make export-all-results

# Or manually
kubectl get configmap <scan-name>-results -o jsonpath='{.data.output}' > result.txt
```

Results are saved to `scan-results/` directory.

---

## 🧹 Cleanup

```bash
# Automated cleanup
make cleanup

# Manual cleanup
kubectl delete clusterscans --all
kubectl delete cronjobs --all
kubectl delete jobs --all
make undeploy
make uninstall
kind delete cluster --name clusterscan
```

---

## 🛠️ Development

```bash
# Run tests
make test

# Run controller locally
make install
make run

# View logs
kubectl logs -n clusterscan-operator-system deployment/clusterscan-operator-controller-manager
```

---

## 📋 API Reference

### ClusterScan Spec

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Scanner image (default: `aquasec/trivy:latest`) |
| `target` | string | Image to scan (required if no command) |
| `command` | []string | Custom command (overrides default) |
| `schedule` | string | Cron schedule (omit for one-time) |
| `suspend` | bool | Pause scheduled scans |

### ClusterScan Status

| Field | Description |
|-------|-------------|
| `phase` | Pending, Running, Completed, or Failed |
| `lastRunTime` | Last execution timestamp |
| `resultsConfigMap` | Name of ConfigMap with results |
| `exitCode` | Exit code of last run |

---

## 🎯 Common Commands

| Command | Description |
|---------|-------------|
| `make setup` | Full automated setup |
| `make demo` | Interactive demo |
| `make cleanup` | Remove all resources |
| `make test` | Run tests |
| `make show-scans` | List all scans |
| `make export-results SCAN=<name>` | Export results |

---

Built with [Kubebuilder](https://book.kubebuilder.io/)

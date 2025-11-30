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

### What it does internally:

```bash
# 1. Creates Kind cluster
kind create cluster --name clusterscan

# 2. Installs cert-manager (needed for webhook TLS certificates)
kubectl apply -f cert-manager.yaml
kubectl wait --for=condition=available deployment/cert-manager

# 3. Generates CRDs and RBAC manifests from Go code
make manifests

# 4. Installs CRDs (ClusterScan definition) into cluster
make install

# 5. Builds Docker image
make docker-build IMG=controller:latest

# 6. Loads image into Kind
kind load docker-image controller:latest --name clusterscan

# 7. Deploys operator to cluster
make deploy IMG=controller:latest

# 8. Waits for operator to be ready
kubectl wait --for=condition=available --timeout=120s \
  deployment/clusterscan-operator-controller-manager -n clusterscan-operator-system
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
| `make export-all-results` | Export all results |

---

Built with [Kubebuilder](https://book.kubebuilder.io/)

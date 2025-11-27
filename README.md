🛡️ ClusterScan Operator

A production-ready Kubernetes Operator for managing security scans (Vulnerability, Compliance) as first-class citizens.

🚀 Overview

ClusterScan abstracts raw CronJobs into a declarative API, handling validation, scheduling, and lifecycle management automatically.

✨ Key Features

🧠 Smart Scheduling: Auto-generates Jobs (one-off) or CronJobs (recurring).

👮 Admission Control: Webhooks validate Cron syntax and auto-fill default security commands.

📊 Observability: Tracks Phase (Pending, Scheduled, Running) and LastJobName.

♻️ Self-Healing: OwnerReferences ensure garbage collection; reconciliation fixes drift.

🏗️ Design Decisions

Validation via Webhooks: We reject invalid configs (e.g., bad cron syntax) at the API level ("Shift Left") rather than handling errors in the controller.

In-Cluster Deployment: The operator runs as a Pod with Cert-Manager handling Webhook TLS, mirroring production architecture.

Idempotent Metrics: We intentionally track LastJobName instead of a TotalRuns counter to prevent reconciliation loops and double-counting during restarts.

⚡ Quick Start

1. Setup Cluster & Cert-Manager

# Create cluster & install Cert-Manager (Required for Webhook TLS)
kind create cluster --name clusterscan-cluster
kubectl apply -f [https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml](https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml)
kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=180s



2. Build & Deploy

# Build, Load to Kind, and Deploy
make docker-build IMG=clusterscan:v1
kind load docker-image clusterscan:v1 --name clusterscan-cluster
make deploy IMG=clusterscan:v1

# Verify
kubectl get pods -n clusterscan-operator-system --watch



🧪 Live Demos

files located in samples/ directory.

1. Mutation (Auto-Fill Defaults)

Applying a spec with only an image automatically injects the trivy command.

kubectl apply -f samples/demo-default.yaml
kubectl get clusterscan demo-auto-fix -o yaml
# Result: command: ["trivy", "image", "nginx:1.19"]



2. Validation (Block Bad Configs)

Applying an invalid Cron schedule is rejected immediately.

kubectl apply -f samples/demo-fail.yaml
# Result: Error... admission webhook denied the request: invalid cron schedule format



3. Real World Compliance

Schedule a nightly CIS Benchmark.

kubectl apply -f samples/real-world-scans.yaml
kubectl get clusterscans
# Result: Phase: Scheduled



📂 Project Structure

api/v1alpha1: CRD definitions

internal/controller: Reconciliation logic

internal/webhook: Webhook logic (Mutation/Validation)

samples/: Demo manifests

Dockerfile: Multi-stage build (distroless)

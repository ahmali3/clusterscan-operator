#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SKIP_CONFIRM=false
SKIP_CLUSTER=false
CLUSTER_ONLY=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -y|--yes) SKIP_CONFIRM=true; shift ;;
        --skip-cluster) SKIP_CLUSTER=true; shift ;;
        --cluster-only) CLUSTER_ONLY=true; shift ;;
        *) shift ;;
    esac
done

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

confirm() {
    if [ "$SKIP_CONFIRM" = true ]; then
        return 0
    fi
    echo -ne "${YELLOW}$1 (y/N): ${NC}"
    read -r response
    [[ "$response" =~ ^[Yy]$ ]]
}

echo ""
echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║       ClusterScan Operator - Setup Script                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Create Kind cluster
if [ "$SKIP_CLUSTER" = false ]; then
    if kind get clusters 2>/dev/null | grep -q "^clusterscan$"; then
        log_info "Kind cluster 'clusterscan' already exists"
        if confirm "Delete and recreate cluster?"; then
            kind delete cluster --name clusterscan
            kind create cluster --name clusterscan
            log_success "Recreated Kind cluster"
        fi
    else
        log_info "Creating Kind cluster..."
        kind create cluster --name clusterscan
        log_success "Created Kind cluster"
    fi
    
    # Install cert-manager
    log_info "Installing cert-manager..."
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
    
    log_info "Waiting for cert-manager to be ready (this takes ~60 seconds)..."
    kubectl wait --for=condition=available --timeout=120s \
        deployment/cert-manager -n cert-manager
    kubectl wait --for=condition=available --timeout=120s \
        deployment/cert-manager-webhook -n cert-manager
    kubectl wait --for=condition=available --timeout=120s \
        deployment/cert-manager-cainjector -n cert-manager
    
    log_success "cert-manager installed and ready"
fi

if [ "$CLUSTER_ONLY" = true ]; then
    log_success "Cluster setup complete!"
    exit 0
fi

# Install CRDs
log_info "Installing CRDs..."
make install
log_success "CRDs installed"

# Build and deploy operator
log_info "Building operator image..."
make docker-build IMG=controller:latest
log_success "Built operator image"

log_info "Loading image into Kind..."
kind load docker-image controller:latest --name clusterscan
log_success "Loaded image"

log_info "Deploying operator..."
make deploy IMG=controller:latest
log_success "Operator deployed"

# Wait for operator
log_info "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=120s \
    deployment/clusterscan-operator-controller-manager \
    -n clusterscan-operator-system

log_success "Operator is ready!"

echo ""
log_success "Setup complete!"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "  • Run demo:          make demo"
echo "  • Apply samples:     kubectl apply -f samples/"
echo "  • View scans:        make show-scans"
echo "  • Clean up:          make cleanup"
echo ""

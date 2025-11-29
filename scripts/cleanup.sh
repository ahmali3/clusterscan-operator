#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SKIP_CONFIRM=false
QUICK_MODE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -y|--yes) SKIP_CONFIRM=true; shift ;;
        --quick) QUICK_MODE=true; shift ;;
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
echo -e "${BLUE}║         ClusterScan Operator - Cleanup Script              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Delete ClusterScan resources
if kubectl get clusterscans &>/dev/null; then
    SCAN_COUNT=$(kubectl get clusterscans --no-headers 2>/dev/null | wc -l)
    if [ "$SCAN_COUNT" -gt 0 ]; then
        log_info "Found $SCAN_COUNT ClusterScan resources"
        kubectl get clusterscans
        echo ""
        if confirm "Delete all ClusterScan resources?"; then
            kubectl delete clusterscans --all
            log_success "Deleted all ClusterScans"
        fi
    fi
fi

# Delete ConfigMaps
if kubectl get configmaps -l app=clusterscan &>/dev/null; then
    CM_COUNT=$(kubectl get configmaps -l app=clusterscan --no-headers 2>/dev/null | wc -l)
    if [ "$CM_COUNT" -gt 0 ]; then
        log_info "Found $CM_COUNT scan result ConfigMaps"
        if confirm "Delete scan result ConfigMaps?"; then
            kubectl delete configmaps -l app=clusterscan
            log_success "Deleted result ConfigMaps"
        fi
    fi
fi

# Delete Jobs
if kubectl get jobs &>/dev/null; then
    JOB_COUNT=$(kubectl get jobs --no-headers 2>/dev/null | wc -l)
    if [ "$JOB_COUNT" -gt 0 ]; then
        log_info "Found $JOB_COUNT Jobs"
        if confirm "Delete all Jobs?"; then
            kubectl delete jobs --all
            log_success "Deleted all Jobs"
        fi
    fi
fi

# Delete CronJobs
if kubectl get cronjobs &>/dev/null; then
    CRON_COUNT=$(kubectl get cronjobs --no-headers 2>/dev/null | wc -l)
    if [ "$CRON_COUNT" -gt 0 ]; then
        log_info "Found $CRON_COUNT CronJobs"
        if confirm "Delete all CronJobs?"; then
            kubectl delete cronjobs --all
            log_success "Deleted all CronJobs"
        fi
    fi
fi

# Clean local results
if [ -d "scan-results" ] && [ "$(ls -A scan-results/*.txt 2>/dev/null)" ]; then
    log_info "Found local scan results"
    if confirm "Delete local scan result files?"; then
        rm -f scan-results/*.txt
        log_success "Deleted local results"
    fi
fi

# Full cleanup (unless quick mode)
if [ "$QUICK_MODE" = false ]; then
    echo ""
    log_info "Full cleanup options:"
    
    if confirm "Uninstall operator from cluster?"; then
        make undeploy 2>/dev/null || log_warn "Operator not deployed"
        make uninstall 2>/dev/null || log_warn "CRDs not installed"
        log_success "Operator uninstalled"
    fi
    
    if confirm "Delete Kind cluster?"; then
        if kind get clusters | grep -q "clusterscan"; then
            kind delete cluster --name clusterscan
            log_success "Deleted Kind cluster"
        else
            log_warn "No Kind cluster named 'clusterscan' found"
        fi
    fi
fi

echo ""
log_success "Cleanup complete!"
echo ""

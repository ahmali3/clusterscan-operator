#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

SAMPLES_DIR="samples"
RESULTS_DIR="scan-results"

clear_screen() {
    clear
    echo -e "${BLUE}${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}${BOLD}║         ClusterScan Operator - Interview Demo             ║${NC}"
    echo -e "${BLUE}${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

pause() {
    echo ""
    echo -e "${CYAN}Press Enter to continue...${NC}"
    read -r
}

run_command() {
    echo -e "${YELLOW}$ $1${NC}"
    eval "$1"
    echo ""
}

wait_for_scan() {
    local scan_name=$1
    local max_wait=${2:-60}
    
    echo -e "${BOLD}Waiting for scan to complete (max ${max_wait}s)...${NC}"
    
    for i in $(seq 1 $max_wait); do
        phase=$(kubectl get clusterscan "$scan_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        
        if [[ "$phase" == "Completed" || "$phase" == "Failed" ]]; then
            echo -e "${GREEN}✓ Scan finished with phase: $phase (${i}s)${NC}"
            return 0
        fi
        
        echo -ne "  ${CYAN}[$i/${max_wait}s]${NC} Phase: ${phase:-Pending}...\r"
        sleep 1
    done
    
    echo ""
    echo -e "${YELLOW}⚠ Scan still running after ${max_wait}s${NC}"
    return 0
}

check_prerequisites() {
    clear_screen
    echo -e "${BOLD}Checking prerequisites...${NC}"
    echo ""
    
    if ! kubectl cluster-info &>/dev/null; then
        echo -e "${RED}❌ No Kubernetes cluster found${NC}"
        echo "Please run: make setup"
        exit 1
    fi
    
    if ! kubectl get crd clusterscans.scan.ahmali3.github.io &>/dev/null; then
        echo -e "${RED}❌ ClusterScan CRD not installed${NC}"
        echo "Please run: make setup"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Kubernetes cluster ready${NC}"
    echo -e "${GREEN}✅ ClusterScan CRD installed${NC}"
    echo -e "${GREEN}✅ Prerequisites met${NC}"
    pause
}

demo_simple_scan() {
    clear_screen
    echo -e "${BOLD}Demo 1: Simple One-Time Scan${NC}"
    echo -e "${CYAN}This demonstrates a basic vulnerability scan of nginx:latest${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying simple scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/00-simple-scan.yaml"
    
    wait_for_scan "simple-scan" 60
    
    echo ""
    echo -e "${BOLD}Checking scan status...${NC}"
    run_command "kubectl get clusterscan simple-scan -o wide"
    
    echo ""
    echo -e "${BOLD}Viewing scan details...${NC}"
    run_command "kubectl describe clusterscan simple-scan | tail -20"
    
    pause
}

demo_webhook_mutation() {
    clear_screen
    echo -e "${BOLD}Demo 2: Webhook Mutation (Default Values)${NC}"
    echo -e "${CYAN}Shows how webhook automatically adds default scanner when omitted${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying scan without image specified...${NC}"
    run_command "cat ${SAMPLES_DIR}/01-default-mutation.yaml"
    echo ""
    run_command "kubectl apply -f ${SAMPLES_DIR}/01-default-mutation.yaml"
    
    echo ""
    echo -e "${BOLD}Checking mutated spec (image was added by webhook)...${NC}"
    run_command "kubectl get clusterscan default-mutation-test -o jsonpath='{.spec.image}' && echo"
    
    pause
}

demo_webhook_validation() {
    clear_screen
    echo -e "${BOLD}Demo 3: Webhook Validation (Invalid Schedule)${NC}"
    echo -e "${CYAN}Shows how webhook rejects invalid cron schedules${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Attempting to apply invalid schedule...${NC}"
    run_command "cat ${SAMPLES_DIR}/02-validation-fail.yaml"
    echo ""
    echo -e "${YELLOW}This should fail validation:${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/02-validation-fail.yaml 2>&1 || true"
    
    pause
}

demo_scheduled_scan() {
    clear_screen
    echo -e "${BOLD}Demo 4: Scheduled Scans (CronJob)${NC}"
    echo -e "${CYAN}Creates a CronJob that runs security scans on a schedule${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying scheduled scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/03-scheduled.yaml"
    
    echo ""
    echo -e "${BOLD}Verifying CronJob was created...${NC}"
    run_command "kubectl get cronjobs"
    
    echo ""
    echo -e "${BOLD}Showing CronJob details...${NC}"
    run_command "kubectl get cronjob daily-security-scan-cron -o yaml | head -30"
    
    echo ""
    echo -e "${BOLD}Demonstrating suspend functionality...${NC}"
    run_command "kubectl patch clusterscan daily-security-scan --type=merge -p '{\"spec\":{\"suspend\":true}}'"
    
    echo ""
    run_command "kubectl get cronjob daily-security-scan-cron -o jsonpath='{.spec.suspend}' && echo"
    
    pause
}

demo_vulnerability_scan() {
    clear_screen
    echo -e "${BOLD}Demo 5: Vulnerability Detection${NC}"
    echo -e "${CYAN}Scans an old Python image with known CVEs${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying vulnerability scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/04-vulnerability-scan.yaml"
    
    wait_for_scan "vulnerable-python-scan" 90
    
    echo ""
    echo -e "${BOLD}Checking scan status...${NC}"
    run_command "kubectl get clusterscan vulnerable-python-scan -o wide"
    
    pause
}

demo_compliance_scan() {
    clear_screen
    echo -e "${BOLD}Demo 6: Compliance Scanning (Kube-bench)${NC}"
    echo -e "${CYAN}Runs CIS Kubernetes benchmark checks${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying compliance scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/05-compliance-scan.yaml"
    
    wait_for_scan "kube-bench-compliance" 60
    
    echo ""
    echo -e "${BOLD}Checking scan status...${NC}"
    run_command "kubectl get clusterscan kube-bench-compliance -o wide"
    
    pause
}

demo_export_results() {
    clear_screen
    echo -e "${BOLD}Demo 7: Exporting Results${NC}"
    echo -e "${CYAN}Shows how to save scan results locally${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Listing all scans...${NC}"
    run_command "make show-scans"
    
    # Get first available scan
    FIRST_SCAN=$(kubectl get clusterscans -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$FIRST_SCAN" ]; then
        echo -e "${YELLOW}No scans available yet. Creating a simple scan first...${NC}"
        run_command "kubectl apply -f ${SAMPLES_DIR}/00-simple-scan.yaml"
        
        echo ""
        echo -e "${BOLD}Waiting for scan to start...${NC}"
        sleep 5
        
        FIRST_SCAN="simple-scan"
    fi
    
    echo ""
    echo -e "${BOLD}Exporting scan: ${FIRST_SCAN}${NC}"
    run_command "make export-results SCAN=${FIRST_SCAN} DIR=${RESULTS_DIR}"
    
    echo ""
    echo -e "${BOLD}Viewing exported files...${NC}"
    run_command "ls -lh ${RESULTS_DIR}/ | tail -5"
    
    echo ""
    echo -e "${BOLD}Showing file preview...${NC}"
    LATEST_FILE=$(ls -t ${RESULTS_DIR}/result_*.txt 2>/dev/null | head -1)
    if [ -n "$LATEST_FILE" ]; then
        run_command "head -30 $LATEST_FILE"
    else
        echo -e "${YELLOW}No result files found yet (scan may still be running)${NC}"
    fi
    
    pause
}

show_summary() {
    clear_screen
    echo -e "${BOLD}Demo Complete! Summary of Features:${NC}"
    echo ""
    echo -e "${GREEN}✅ One-time scans${NC} - Run security scans on-demand"
    echo -e "${GREEN}✅ Scheduled scans${NC} - Automated CronJob-based scanning"
    echo -e "${GREEN}✅ Webhook validation${NC} - Reject invalid configurations"
    echo -e "${GREEN}✅ Webhook mutation${NC} - Auto-populate default values"
    echo -e "${GREEN}✅ Multiple scanners${NC} - Trivy, Kube-bench support"
    echo -e "${GREEN}✅ Result export${NC} - Save results locally with timestamps"
    echo -e "${GREEN}✅ Phase tracking${NC} - Running → Completed/Failed states"
    echo -e "${GREEN}✅ Suspend/Resume${NC} - Dynamic CronJob control"
    echo ""
    echo -e "${CYAN}Next Steps:${NC}"
    echo "  • View all scans:     make show-scans"
    echo "  • Export results:     make export-all-results"
    echo "  • Clean up:           make cleanup"
    echo ""
}

main_menu() {
    while true; do
        clear_screen
        echo -e "${BOLD}Select a demo:${NC}"
        echo ""
        echo "  1) Simple One-Time Scan"
        echo "  2) Webhook Mutation (Defaults)"
        echo "  3) Webhook Validation (Invalid Schedule)"
        echo "  4) Scheduled Scans (CronJob)"
        echo "  5) Vulnerability Detection"
        echo "  6) Compliance Scanning (Kube-bench)"
        echo "  7) Export Results"
        echo "  8) Run All Demos"
        echo "  9) Show Summary"
        echo "  0) Exit"
        echo ""
        echo -ne "${CYAN}Enter choice: ${NC}"
        read -r choice
        
        case $choice in
            1) demo_simple_scan ;;
            2) demo_webhook_mutation ;;
            3) demo_webhook_validation ;;
            4) demo_scheduled_scan ;;
            5) demo_vulnerability_scan ;;
            6) demo_compliance_scan ;;
            7) demo_export_results ;;
            8)
                demo_simple_scan
                demo_webhook_mutation
                demo_webhook_validation
                demo_scheduled_scan
                demo_vulnerability_scan
                demo_compliance_scan
                demo_export_results
                show_summary
                ;;
            9) show_summary && pause ;;
            0) 
                echo ""
                echo -e "${GREEN}Thanks for watching the demo!${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid choice${NC}"
                sleep 1
                ;;
        esac
    done
}

# Main execution
check_prerequisites
main_menu

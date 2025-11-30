#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SAMPLES_DIR="${PROJECT_ROOT}/samples"
RESULTS_DIR="${PROJECT_ROOT}/scan-results"

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

clear_screen() {
    clear
}

pause() {
    echo ""
    echo -e "${CYAN}Press Enter to continue...${NC}"
    read -r
}

run_command() {
    echo -e "${GREEN}$ $1${NC}"
    eval "$1"
}

show_menu() {
    clear_screen
    echo -e "${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║         ClusterScan Operator - Interview Demo             ║${NC}"
    echo -e "${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Choose a demo scenario:"
    echo ""
    echo "  1) Simple One-Time Scan"
    echo "  2) Webhook Mutation (Default Values)"
    echo "  3) Webhook Validation (Invalid Schedule)"
    echo "  4) Scheduled Scans (CronJob)"
    echo "  5) Vulnerability Detection"
    echo "  6) Compliance Scanning"
    echo "  7) Export Results"
    echo "  8) Run All Demos"
    echo "  9) Exit"
    echo ""
    echo -n "Enter choice: "
}

demo_simple_scan() {
    clear_screen
    echo -e "${BOLD}Demo 1: Simple One-Time Scan${NC}"
    echo -e "${CYAN}Creates a basic Trivy scan that runs once${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying simple scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/00-simple-scan.yaml"
    
    echo ""
    echo -e "${BOLD}Watching scan progress...${NC}"
    sleep 3
    run_command "kubectl get clusterscan simple-scan"
    
    echo ""
    echo -e "${BOLD}Checking if Job was created...${NC}"
    run_command "kubectl get jobs | grep simple-scan || echo 'Job not created yet...'"
    
    echo ""
    echo -e "${BOLD}Viewing ClusterScan details...${NC}"
    run_command "kubectl get clusterscan simple-scan -o yaml | head -40"
    
    pause
}

demo_webhook_mutation() {
    clear_screen
    echo -e "${BOLD}Demo 2: Webhook Mutation (Default Values)${NC}"
    echo -e "${CYAN}Shows how webhook automatically fills in defaults${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Viewing YAML before mutation...${NC}"
    run_command "cat ${SAMPLES_DIR}/01-default-mutation.yaml"
    
    echo ""
    echo -e "${YELLOW}Note: No 'image' field specified!${NC}"
    pause
    
    echo ""
    echo -e "${BOLD}Applying scan (webhook will add defaults)...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/01-default-mutation.yaml"
    
    echo ""
    echo -e "${BOLD}Viewing YAML after mutation...${NC}"
    run_command "kubectl get clusterscan default-mutation-test -o yaml | grep -A5 'spec:'"
    
    echo ""
    echo -e "${GREEN}✓ Webhook automatically added: image: aquasec/trivy:latest${NC}"
    echo -e "${GREEN}✓ Webhook automatically added: command: [trivy, image, nginx:latest]${NC}"
    
    pause
}

demo_webhook_validation() {
    clear_screen
    echo -e "${BOLD}Demo 3: Webhook Validation (Invalid Schedule)${NC}"
    echo -e "${CYAN}Shows how webhook rejects invalid configurations${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Attempting to create scan with invalid schedule...${NC}"
    run_command "cat ${SAMPLES_DIR}/02-validation-fail.yaml"
    
    echo ""
    echo -e "${BOLD}Applying (this should fail validation)...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/02-validation-fail.yaml || true"
    
    echo ""
    echo -e "${YELLOW}As expected, the webhook rejected the invalid configuration!${NC}"
    
    pause
}

demo_scheduled_scans() {
    clear_screen
    echo -e "${BOLD}Demo 4: Scheduled Scans (CronJob)${NC}"
    echo -e "${CYAN}Creates a CronJob that runs security scans on a schedule${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying scheduled scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/03-scheduled.yaml"
    
    echo ""
    echo -e "${BOLD}Waiting for CronJob to be created...${NC}"
    sleep 3
    
    echo ""
    echo -e "${BOLD}Verifying CronJob was created...${NC}"
    run_command "kubectl get cronjobs"
    
    echo ""
    echo -e "${BOLD}Showing CronJob details...${NC}"
    run_command "kubectl get cronjob scheduled-scan-cron -o yaml | head -30"
    
    echo ""
    echo -e "${BOLD}Demonstrating suspend functionality...${NC}"
    run_command "kubectl patch clusterscan scheduled-scan --type=merge -p '{\"spec\":{\"suspend\":true}}'"
    
    echo ""
    echo -e "${BOLD}Verifying CronJob is suspended...${NC}"
    sleep 2
    run_command "kubectl get cronjob scheduled-scan-cron -o jsonpath='{.spec.suspend}'"
    echo ""
    
    echo ""
    echo -e "${BOLD}Resuming the scan...${NC}"
    run_command "kubectl patch clusterscan scheduled-scan --type=merge -p '{\"spec\":{\"suspend\":false}}'"
    
    echo ""
    echo -e "${BOLD}Verifying CronJob is resumed...${NC}"
    sleep 2
    run_command "kubectl get cronjob scheduled-scan-cron -o jsonpath='{.spec.suspend}'"
    echo ""
    
    pause
}

demo_vulnerability_detection() {
    clear_screen
    echo -e "${BOLD}Demo 5: Vulnerability Detection${NC}"
    echo -e "${CYAN}Scans an older Python image with known CVEs${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying vulnerability scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/04-vulnerability-scan.yaml"
    
    echo ""
    echo -e "${BOLD}Waiting for scan to start...${NC}"
    sleep 5
    
    echo ""
    echo -e "${BOLD}Checking scan status...${NC}"
    run_command "kubectl get clusterscan vulnerability-scan"
    
    echo ""
    echo -e "${BOLD}Showing job details...${NC}"
    run_command "kubectl get jobs | grep vulnerability-scan"
    
    echo ""
    echo -e "${YELLOW}Note: Scan may take 30-60 seconds to find vulnerabilities${NC}"
    echo -e "${YELLOW}Check results later with: make export-results SCAN=vulnerability-scan${NC}"
    
    pause
}

demo_compliance_scanning() {
    clear_screen
    echo -e "${BOLD}Demo 6: Compliance Scanning${NC}"
    echo -e "${CYAN}Runs CIS Kubernetes Benchmark checks${NC}"
    echo ""
    pause
    
    echo -e "${BOLD}Applying compliance scan...${NC}"
    run_command "kubectl apply -f ${SAMPLES_DIR}/05-compliance-scan.yaml"
    
    echo ""
    echo -e "${BOLD}Waiting for scan to start...${NC}"
    sleep 5
    
    echo ""
    echo -e "${BOLD}Checking scan status...${NC}"
    run_command "kubectl get clusterscan compliance-scan"
    
    echo ""
    echo -e "${BOLD}Showing job details...${NC}"
    run_command "kubectl get jobs | grep compliance-scan"
    
    echo ""
    echo -e "${YELLOW}Note: Kube-bench checks control plane and node configurations${NC}"
    echo -e "${YELLOW}Check results with: kubectl logs job/compliance-scan-job${NC}"
    
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
    
    echo ""
    echo -e "${BOLD}Exporting ALL scan results...${NC}"
    run_command "make export-all-results DIR=${RESULTS_DIR}"
    
    echo ""
    echo -e "${BOLD}Viewing all exported files...${NC}"
    run_command "ls -lh ${RESULTS_DIR}/"
    
    echo ""
    echo -e "${BOLD}Counting exported results...${NC}"
    RESULT_COUNT=$(ls ${RESULTS_DIR}/result_*.txt 2>/dev/null | wc -l | xargs)
    echo -e "${GREEN}✓ Exported ${RESULT_COUNT} scan results${NC}"
    
    echo ""
    echo -e "${BOLD}Showing preview of latest result...${NC}"
    LATEST_FILE=$(ls -t ${RESULTS_DIR}/result_*.txt 2>/dev/null | head -1)
    if [ -n "$LATEST_FILE" ]; then
        echo -e "${CYAN}File: $(basename $LATEST_FILE)${NC}"
        run_command "head -30 $LATEST_FILE"
    else
        echo -e "${YELLOW}No result files found yet (scans may still be running)${NC}"
    fi
    
    echo ""
    echo -e "${BOLD}You can also export a single scan:${NC}"
    echo -e "${CYAN}make export-results SCAN=simple-scan${NC}"
    
    pause
}

run_all_demos() {
    demo_simple_scan
    demo_webhook_mutation
    demo_webhook_validation
    demo_scheduled_scans
    demo_vulnerability_detection
    demo_compliance_scanning
    demo_export_results
    
    clear_screen
    echo -e "${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║              All Demos Completed!                          ║${NC}"
    echo -e "${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}✓ Demonstrated one-time scans${NC}"
    echo -e "${GREEN}✓ Demonstrated webhook mutation${NC}"
    echo -e "${GREEN}✓ Demonstrated webhook validation${NC}"
    echo -e "${GREEN}✓ Demonstrated scheduled scans${NC}"
    echo -e "${GREEN}✓ Demonstrated vulnerability scanning${NC}"
    echo -e "${GREEN}✓ Demonstrated compliance scanning${NC}"
    echo -e "${GREEN}✓ Demonstrated result export${NC}"
    echo ""
    echo -e "${CYAN}View all scans: kubectl get clusterscans${NC}"
    echo -e "${CYAN}Clean up: make cleanup${NC}"
    echo ""
    pause
}

main() {
    if [ ! -f "${SAMPLES_DIR}/00-simple-scan.yaml" ]; then
        echo "Error: Sample files not found in ${SAMPLES_DIR}"
        exit 1
    fi
    
    mkdir -p "${RESULTS_DIR}"
    
    while true; do
        show_menu
        read -r choice
        
        case $choice in
            1) demo_simple_scan ;;
            2) demo_webhook_mutation ;;
            3) demo_webhook_validation ;;
            4) demo_scheduled_scans ;;
            5) demo_vulnerability_detection ;;
            6) demo_compliance_scanning ;;
            7) demo_export_results ;;
            8) run_all_demos ;;
            9) 
                echo ""
                echo "Exiting demo. Thanks!"
                exit 0
                ;;
            *)
                echo ""
                echo "Invalid choice. Press Enter to continue..."
                read -r
                ;;
        esac
    done
}

main

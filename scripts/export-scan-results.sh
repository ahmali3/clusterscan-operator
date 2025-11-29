#!/bin/bash

# Exit on errors
set -e

# Terminal colors
C_ERROR='\033[0;31m'
C_SUCCESS='\033[0;32m'
C_WARN='\033[1;33m'
C_INFO='\033[0;34m'
C_RESET='\033[0m'

# Configuration
RESULTS_DIR="${2:-./scan-results}"
SCAN_RESOURCE="$1"

show_usage() {
    echo "Export ClusterScan Results"
    echo ""
    echo "USAGE:"
    echo "  $0 <scan-name> [output-dir]"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 my-trivy-scan"
    echo "  $0 my-trivy-scan /tmp/exports"
    echo ""
    echo "BATCH EXPORT:"
    echo "  kubectl get clusterscans -o name | while read scan; do"
    echo "    name=\$(basename \$scan)"
    echo "    $0 \"\$name\""
    echo "  done"
    exit 1
}

log_info() {
    echo -e "${C_INFO}[INFO]${C_RESET} $1"
}

log_success() {
    echo -e "${C_SUCCESS}[SUCCESS]${C_RESET} $1"
}

log_error() {
    echo -e "${C_ERROR}[ERROR]${C_RESET} $1"
}

log_warn() {
    echo -e "${C_WARN}[WARN]${C_RESET} $1"
}

# Validate input
if [[ -z "$SCAN_RESOURCE" ]]; then
    log_error "Scan name not provided"
    show_usage
fi

log_info "Processing scan: $SCAN_RESOURCE"

# Verify resource exists
if ! kubectl get clusterscan "$SCAN_RESOURCE" &>/dev/null; then
    log_error "ClusterScan '$SCAN_RESOURCE' does not exist"
    echo ""
    echo "Available resources:"
    kubectl get clusterscans --no-headers | awk '{print "  - " $1}'
    exit 1
fi

# Extract metadata
extract_field() {
    kubectl get clusterscan "$SCAN_RESOURCE" -o jsonpath="{$1}" 2>/dev/null || echo "$2"
}

SCANNER_IMAGE=$(extract_field '.spec.image' 'unknown')
SCAN_TARGET=$(extract_field '.spec.target' '')
CRON_SCHEDULE=$(extract_field '.spec.schedule' '')
STATUS_PHASE=$(extract_field '.status.phase' 'Unknown')
RESULT_EXIT=$(extract_field '.status.scanExitCode' 'N/A')
RESULT_CM=$(extract_field '.status.resultsConfigMap' '')
COMPLETION_TIME=$(extract_field '.status.lastRunTime' 'N/A')

# Generate output filename
EXPORT_TIME=$(date +%Y%m%d_%H%M%S)
EXPORT_DATE_FULL=$(date '+%Y-%m-%d %H:%M:%S %Z')

# Sanitize for filesystem
sanitize() {
    echo "$1" | tr '/:.' '_' | tr -s '_'
}

CLEAN_IMAGE=$(sanitize "$SCANNER_IMAGE")
CLEAN_TARGET=$(sanitize "$SCAN_TARGET")

if [[ -n "$SCAN_TARGET" ]]; then
    OUTPUT_FILE="${RESULTS_DIR}/result_${CLEAN_IMAGE}_${CLEAN_TARGET}_${EXPORT_TIME}.txt"
else
    OUTPUT_FILE="${RESULTS_DIR}/result_${CLEAN_IMAGE}_${EXPORT_TIME}.txt"
fi

# Create directory
mkdir -p "$RESULTS_DIR"

# Validate scan state
if [[ "$STATUS_PHASE" != "Completed" && "$STATUS_PHASE" != "Failed" ]]; then
    log_warn "Scan phase is '$STATUS_PHASE' - results may be unavailable"
fi

# Check for results
if [[ -z "$RESULT_CM" ]]; then
    log_error "No ConfigMap found for results"
    echo ""
    echo "This may indicate:"
    echo "  • Scan is still running (phase: $STATUS_PHASE)"
    echo "  • Scan failed before generating output"
    echo "  • Results were removed manually"
    echo ""
    echo "Debug with: kubectl describe clusterscan $SCAN_RESOURCE"
    exit 1
fi

log_info "Retrieving data from ConfigMap: $RESULT_CM"

# Fetch scan output
SCAN_DATA=$(kubectl get configmap "$RESULT_CM" -o jsonpath='{.data.scan-output\.txt}' 2>/dev/null)

if [[ -z "$SCAN_DATA" ]]; then
    log_warn "ConfigMap found but contains no output data"
fi

# Write file with header
{
    echo "================================================================================"
    echo "CLUSTERSCAN EXPORT"
    echo "================================================================================"
    echo "Resource:        $SCAN_RESOURCE"
    echo "Scanner:         $SCANNER_IMAGE"
    echo "Target:          ${SCAN_TARGET:-None}"
    echo "Schedule:        ${CRON_SCHEDULE:-One-time execution}"
    echo "Status:          $STATUS_PHASE"
    echo "Exit Code:       $RESULT_EXIT"
    echo "Completed:       $COMPLETION_TIME"
    echo "Exported:        $EXPORT_DATE_FULL"
    echo "Source CM:       $RESULT_CM"
    echo "================================================================================"
    echo ""
    echo "$SCAN_DATA"
} > "$OUTPUT_FILE"

# Calculate statistics
BYTE_COUNT=$(wc -c < "$OUTPUT_FILE")
LINE_COUNT=$(wc -l < "$OUTPUT_FILE")

# Format size
if (( BYTE_COUNT > 1048576 )); then
    SIZE_DISPLAY=$(awk "BEGIN {printf \"%.2f MB\", $BYTE_COUNT/1048576}")
elif (( BYTE_COUNT > 1024 )); then
    SIZE_DISPLAY=$(awk "BEGIN {printf \"%.2f KB\", $BYTE_COUNT/1024}")
else
    SIZE_DISPLAY="${BYTE_COUNT} bytes"
fi

# Display results
echo ""
log_success "Export completed successfully"
echo ""
echo "FILE INFORMATION:"
echo "  Location:    $OUTPUT_FILE"
echo "  Size:        $SIZE_DISPLAY"
echo "  Lines:       $LINE_COUNT"
echo ""
echo "SCAN DETAILS:"
echo "  Scanner:     $SCANNER_IMAGE"
echo "  Target:      ${SCAN_TARGET:-N/A}"
echo "  Status:      $STATUS_PHASE"
echo "  Exit Code:   $RESULT_EXIT"
echo "  Completed:   $COMPLETION_TIME"
echo ""
echo "NEXT STEPS:"
echo "  cat $OUTPUT_FILE"
echo "  head -20 $OUTPUT_FILE"
echo "  grep -i error $OUTPUT_FILE"
echo ""

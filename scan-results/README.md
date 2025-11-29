# Scan Results Directory

This directory stores exported scan results from ClusterScan resources.

## Viewing Available Scans

List all ClusterScan resources:
```bash
make show-scans
```

## Exporting Results

Export results using:
```bash
# Single scan
make export-results SCAN=my-scan

# All scans
make export-all-results

# Custom directory
make export-results SCAN=my-scan DIR=/tmp/reports
```

## File Naming

Files are named with the pattern:
```
result_<image>_<target>_<timestamp>.txt
```

Examples:
- `result_aquasec_trivy_latest_nginx_1_19_20241129_083045.txt`
- `result_aquasec_kube-bench_latest_20241129_084512.txt`

## Workflow Example

```bash
# 1. See what scans exist
make show-scans

# 2. Export specific scan
make export-results SCAN=trivy-nginx-scan

# 3. View the results
cat scan-results/result_*.txt

# 4. Or export everything at once
make export-all-results
```

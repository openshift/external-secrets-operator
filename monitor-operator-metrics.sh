#!/bin/bash
# Historical Metrics Monitor for External Secrets Operator
# Tracks CPU and memory over time, detects spikes, and provides statistics

set -e

KUBECONFIG="${KUBECONFIG:-/home/mykastur/gcp_n/install-dir/auth/kubeconfig}"
export KUBECONFIG
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-external-secrets-operator}"

# Configuration
SAMPLE_INTERVAL="${SAMPLE_INTERVAL:-2}"  # seconds between samples
DURATION="${DURATION:-300}"              # total monitoring duration in seconds (default 5 minutes)
OUTPUT_DIR="${OUTPUT_DIR:-/tmp/eso-metrics}"
DATA_FILE="${OUTPUT_DIR}/metrics-$(date +%Y%m%d-%H%M%S).csv"
STATS_FILE="${OUTPUT_DIR}/stats-$(date +%Y%m%d-%H%M%S).txt"

# Spike detection thresholds (percentage increase)
CPU_SPIKE_THRESHOLD="${CPU_SPIKE_THRESHOLD:-50}"      # 50% increase
MEMORY_SPIKE_THRESHOLD="${MEMORY_SPIKE_THRESHOLD:-20}" # 20% increase

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

print_header() {
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
}

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_metric() {
    echo -e "${MAGENTA}📊${NC} $1"
}

print_spike() {
    echo -e "${YELLOW}🔥${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

# Convert memory to MB
mem_to_mb() {
    local mem=$1
    if [[ $mem =~ ([0-9]+)Mi ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ $mem =~ ([0-9]+)Gi ]]; then
        echo "$((${BASH_REMATCH[1]} * 1024))"
    elif [[ $mem =~ ([0-9]+)Ki ]]; then
        echo "$((${BASH_REMATCH[1]} / 1024))"
    elif [[ $mem =~ ^([0-9]+)$ ]]; then
        echo "$1"
    else
        echo "0"
    fi
}

# Convert CPU to millicores
cpu_to_millicores() {
    local cpu=$1
    if [[ $cpu =~ ([0-9]+)m ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ $cpu =~ ([0-9\.]+) ]]; then
        echo "$(echo "${BASH_REMATCH[1]} * 1000" | bc 2>/dev/null || echo "0")"
    else
        echo "0"
    fi
}

# Parse command line arguments
MODE="monitor"
ANALYZE_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        analyze)
            MODE="analyze"
            ANALYZE_FILE="$2"
            shift 2
            ;;
        continuous)
            MODE="continuous"
            shift
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --interval)
            SAMPLE_INTERVAL="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [MODE] [OPTIONS]"
            echo ""
            echo "Modes:"
            echo "  monitor              Run one-time monitoring session (default)"
            echo "  continuous           Run continuously until interrupted"
            echo "  analyze FILE         Analyze existing metrics file"
            echo ""
            echo "Options:"
            echo "  --duration SECONDS   Monitoring duration (default: 300)"
            echo "  --interval SECONDS   Sample interval (default: 2)"
            echo "  --help               Show this help"
            echo ""
            echo "Environment Variables:"
            echo "  CPU_SPIKE_THRESHOLD      CPU spike threshold % (default: 50)"
            echo "  MEMORY_SPIKE_THRESHOLD   Memory spike threshold % (default: 20)"
            echo "  OUTPUT_DIR               Output directory (default: /tmp/eso-metrics)"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Monitor for 5 minutes"
            echo "  $0 --duration 600                     # Monitor for 10 minutes"
            echo "  $0 continuous                         # Monitor continuously"
            echo "  $0 analyze /tmp/eso-metrics/metrics-*.csv"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Get operator pod
get_operator_pod() {
    oc get pod -n "$OPERATOR_NAMESPACE" -l app=external-secrets-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Collect metrics
collect_metrics() {
    local pod=$1
    local timestamp=$(date +%s.%N)
    
    # Get metrics from oc adm top
    local metrics=$(oc adm top pod "$pod" -n "$OPERATOR_NAMESPACE" --no-headers 2>/dev/null || echo "N/A N/A")
    local cpu=$(echo "$metrics" | awk '{print $2}')
    local mem=$(echo "$metrics" | awk '{print $3}')
    
    # Convert to standard units
    local cpu_m=$(cpu_to_millicores "$cpu")
    local mem_mb=$(mem_to_mb "$mem")
    
    echo "$timestamp,$cpu_m,$mem_mb"
}

# Analyze metrics file
analyze_metrics() {
    local file=$1
    
    if [ ! -f "$file" ]; then
        echo "Error: File not found: $file"
        exit 1
    fi
    
    print_header "Metrics Analysis: $(basename $file)"
    echo ""
    
    # Skip header if present
    local data=$(grep -v "^timestamp,cpu,memory" "$file" | grep -v "^#")
    
    if [ -z "$data" ]; then
        echo "Error: No data found in file"
        exit 1
    fi
    
    # Calculate statistics using awk
    local stats=$(echo "$data" | awk -F',' '
    BEGIN {
        min_cpu = 999999
        max_cpu = 0
        sum_cpu = 0
        min_mem = 999999
        max_mem = 0
        sum_mem = 0
        count = 0
        prev_cpu = 0
        prev_mem = 0
        spike_count_cpu = 0
        spike_count_mem = 0
        in_cpu_spike = 0
        in_mem_spike = 0
        cpu_spike_start = 0
        mem_spike_start = 0
        total_cpu_spike_duration = 0
        total_mem_spike_duration = 0
        cpu_spike_threshold = '$CPU_SPIKE_THRESHOLD'
        mem_spike_threshold = '$MEMORY_SPIKE_THRESHOLD'
    }
    {
        timestamp = $1
        cpu = $2
        mem = $3
        
        if (cpu > 0 && mem > 0) {
            # Statistics
            if (cpu < min_cpu) min_cpu = cpu
            if (cpu > max_cpu) max_cpu = cpu
            sum_cpu += cpu
            
            if (mem < min_mem) min_mem = mem
            if (mem > max_mem) max_mem = mem
            sum_mem += mem
            
            count++
            
            # Spike detection
            if (count > 1 && prev_cpu > 0 && cpu > 0) {
                cpu_increase = ((cpu - prev_cpu) / prev_cpu) * 100
                if (cpu_increase > cpu_spike_threshold && cpu_increase != "inf") {
                    if (!in_cpu_spike) {
                        spike_count_cpu++
                        in_cpu_spike = 1
                        cpu_spike_start = timestamp
                        printf "CPU_SPIKE:%s:%.0f:%.0f:%.2f\n", timestamp, prev_cpu, cpu, cpu_increase
                    }
                } else if (in_cpu_spike && cpu_increase < (cpu_spike_threshold / 2)) {
                    in_cpu_spike = 0
                    duration = timestamp - cpu_spike_start
                    total_cpu_spike_duration += duration
                    printf "CPU_SPIKE_END:%s:%.2f\n", timestamp, duration
                }
            }
            
            if (count > 1 && prev_mem > 0 && mem > 0) {
                mem_increase = ((mem - prev_mem) / prev_mem) * 100
                if (mem_increase > mem_spike_threshold && mem_increase != "inf") {
                    if (!in_mem_spike) {
                        spike_count_mem++
                        in_mem_spike = 1
                        mem_spike_start = timestamp
                        printf "MEM_SPIKE:%s:%.0f:%.0f:%.2f\n", timestamp, prev_mem, mem, mem_increase
                    }
                } else if (in_mem_spike && mem_increase < (mem_spike_threshold / 2)) {
                    in_mem_spike = 0
                    duration = timestamp - mem_spike_start
                    total_mem_spike_duration += duration
                    printf "MEM_SPIKE_END:%s:%.2f\n", timestamp, duration
                }
            }
            
            prev_cpu = cpu
            prev_mem = mem
        }
    }
    END {
        if (count > 0) {
            avg_cpu = sum_cpu / count
            avg_mem = sum_mem / count
            avg_cpu_spike_duration = (spike_count_cpu > 0) ? total_cpu_spike_duration / spike_count_cpu : 0
            avg_mem_spike_duration = (spike_count_mem > 0) ? total_mem_spike_duration / spike_count_mem : 0
            
            printf "STATS:%d:%.0f:%.0f:%.0f:%.0f:%.0f:%.0f:%d:%d:%.2f:%.2f\n", 
                count, min_cpu, max_cpu, avg_cpu, min_mem, max_mem, avg_mem,
                spike_count_cpu, spike_count_mem, avg_cpu_spike_duration, avg_mem_spike_duration
        }
    }
    ')
    
    # Parse statistics
    local stats_line=$(echo "$stats" | grep "^STATS:")
    if [ -z "$stats_line" ]; then
        echo "Error: Failed to calculate statistics"
        exit 1
    fi
    
    IFS=':' read -r _ sample_count min_cpu max_cpu avg_cpu min_mem max_mem avg_mem \
        spike_count_cpu spike_count_mem avg_cpu_spike_dur avg_mem_spike_dur <<< "$stats_line"
    
    # Display statistics
    print_header "Overall Statistics"
    print_metric "Total samples: $sample_count"
    print_metric "Duration: $(awk "BEGIN {print $sample_count * $SAMPLE_INTERVAL}") seconds ($(awk "BEGIN {printf \"%.1f\", $sample_count * $SAMPLE_INTERVAL / 60}") minutes)"
    echo ""
    
    print_header "CPU Statistics (millicores)"
    print_metric "Minimum: ${min_cpu}m"
    print_metric "Maximum: ${max_cpu}m"
    print_metric "Average: ${avg_cpu}m"
    print_metric "Range: $(awk "BEGIN {print $max_cpu - $min_cpu}")m"
    if [ "$max_cpu" != "0" ] && [ "$min_cpu" != "0" ]; then
        local cpu_variance=$(awk "BEGIN {printf \"%.1f\", (($max_cpu - $min_cpu) / $min_cpu) * 100}")
        print_metric "Variance: ${cpu_variance}%"
    fi
    echo ""
    
    print_header "Memory Statistics (MB)"
    print_metric "Minimum: ${min_mem}Mi"
    print_metric "Maximum: ${max_mem}Mi"
    print_metric "Average: ${avg_mem}Mi"
    print_metric "Range: $(awk "BEGIN {print $max_mem - $min_mem}")Mi"
    if [ "$max_mem" != "0" ] && [ "$min_mem" != "0" ]; then
        local mem_variance=$(awk "BEGIN {printf \"%.1f\", (($max_mem - $min_mem) / $min_mem) * 100}")
        print_metric "Variance: ${mem_variance}%"
    fi
    echo ""
    
    print_header "Spike Analysis"
    print_metric "CPU spike threshold: ${CPU_SPIKE_THRESHOLD}%"
    print_metric "Memory spike threshold: ${MEMORY_SPIKE_THRESHOLD}%"
    echo ""
    
    print_metric "CPU spikes detected: $spike_count_cpu"
    if [ "$spike_count_cpu" -gt 0 ]; then
        print_metric "Average CPU spike duration: ${avg_cpu_spike_dur}s"
    fi
    echo ""
    
    print_metric "Memory spikes detected: $spike_count_mem"
    if [ "$spike_count_mem" -gt 0 ]; then
        print_metric "Average memory spike duration: ${avg_mem_spike_dur}s"
    fi
    echo ""
    
    # Show spike details
    if [ "$spike_count_cpu" -gt 0 ] || [ "$spike_count_mem" -gt 0 ]; then
        print_header "Spike Details"
        
        if [ "$spike_count_cpu" -gt 0 ]; then
            echo "CPU Spikes:"
            echo "$stats" | grep "^CPU_SPIKE:" | while IFS=':' read -r _ timestamp prev_cpu new_cpu increase; do
                local readable_time=$(date -d "@$(echo $timestamp | cut -d'.' -f1)" '+%H:%M:%S' 2>/dev/null || echo "N/A")
                print_spike "  $readable_time - ${prev_cpu}m → ${new_cpu}m (+${increase}%)"
            done
            echo ""
        fi
        
        if [ "$spike_count_mem" -gt 0 ]; then
            echo "Memory Spikes:"
            echo "$stats" | grep "^MEM_SPIKE:" | while IFS=':' read -r _ timestamp prev_mem new_mem increase; do
                local readable_time=$(date -d "@$(echo $timestamp | cut -d'.' -f1)" '+%H:%M:%S' 2>/dev/null || echo "N/A")
                print_spike "  $readable_time - ${prev_mem}Mi → ${new_mem}Mi (+${increase}%)"
            done
            echo ""
        fi
    fi
    
    # Save statistics to file
    {
        echo "# Metrics Analysis Report"
        echo "# Generated: $(date)"
        echo "# File: $file"
        echo ""
        echo "## Overall Statistics"
        echo "Total samples: $sample_count"
        echo "Duration: $(awk "BEGIN {print $sample_count * $SAMPLE_INTERVAL}") seconds"
        echo ""
        echo "## CPU Statistics (millicores)"
        echo "Minimum: ${min_cpu}m"
        echo "Maximum: ${max_cpu}m"
        echo "Average: ${avg_cpu}m"
        echo ""
        echo "## Memory Statistics (MB)"
        echo "Minimum: ${min_mem}Mi"
        echo "Maximum: ${max_mem}Mi"
        echo "Average: ${avg_mem}Mi"
        echo ""
        echo "## Spike Analysis"
        echo "CPU spikes: $spike_count_cpu"
        echo "Memory spikes: $spike_count_mem"
        echo "Avg CPU spike duration: ${avg_cpu_spike_dur}s"
        echo "Avg Memory spike duration: ${avg_mem_spike_dur}s"
    } > "${file%.csv}-analysis.txt"
    
    print_success "Analysis saved to: ${file%.csv}-analysis.txt"
}

# Monitor mode
if [ "$MODE" = "analyze" ]; then
    if [ -z "$ANALYZE_FILE" ]; then
        echo "Error: No file specified for analysis"
        echo "Usage: $0 analyze <file.csv>"
        exit 1
    fi
    
    analyze_metrics "$ANALYZE_FILE"
    exit 0
fi

# Monitoring mode
print_header "External Secrets Operator - Metrics Monitor"
echo ""
echo "Configuration:"
echo "  Namespace: $OPERATOR_NAMESPACE"
echo "  Sample interval: ${SAMPLE_INTERVAL}s"
if [ "$MODE" = "continuous" ]; then
    echo "  Mode: Continuous (Ctrl+C to stop)"
else
    echo "  Duration: ${DURATION}s ($(awk "BEGIN {printf \"%.1f\", $DURATION / 60}") minutes)"
fi
echo "  CPU spike threshold: ${CPU_SPIKE_THRESHOLD}%"
echo "  Memory spike threshold: ${MEMORY_SPIKE_THRESHOLD}%"
echo "  Output: $DATA_FILE"
echo ""

# Get operator pod
POD=$(get_operator_pod)
if [ -z "$POD" ]; then
    echo "Error: Operator pod not found"
    exit 1
fi

print_success "Monitoring pod: $POD"
echo ""

# Create CSV header
echo "# External Secrets Operator Metrics" > "$DATA_FILE"
echo "# Pod: $POD" >> "$DATA_FILE"
echo "# Started: $(date)" >> "$DATA_FILE"
echo "# Sample interval: ${SAMPLE_INTERVAL}s" >> "$DATA_FILE"
echo "timestamp,cpu_millicores,memory_mb" >> "$DATA_FILE"

print_step "Starting data collection..."
echo ""

# Initialize tracking variables
PREV_CPU=0
PREV_MEM=0
SAMPLE_COUNT=0
IN_CPU_SPIKE=0
IN_MEM_SPIKE=0
CPU_SPIKE_COUNT=0
MEM_SPIKE_COUNT=0

# Calculate end time for monitor mode
if [ "$MODE" != "continuous" ]; then
    END_TIME=$(($(date +%s) + DURATION))
fi

# Signal handler for graceful shutdown
cleanup() {
    echo ""
    echo ""
    print_step "Stopping data collection..."
    print_success "Collected $SAMPLE_COUNT samples"
    print_success "Data saved to: $DATA_FILE"
    echo ""
    
    # Auto-analyze
    if [ "$SAMPLE_COUNT" -gt 0 ]; then
        print_step "Analyzing collected data..."
        echo ""
        analyze_metrics "$DATA_FILE"
    fi
    
    exit 0
}

trap cleanup SIGINT SIGTERM

# Main collection loop
while true; do
    # Check if we should stop (monitor mode only)
    if [ "$MODE" != "continuous" ] && [ $(date +%s) -ge $END_TIME ]; then
        cleanup
    fi
    
    # Collect metrics
    METRICS=$(collect_metrics "$POD")
    
    if [ -n "$METRICS" ] && [ "$METRICS" != "N/A" ]; then
        IFS=',' read -r TIMESTAMP CPU MEM <<< "$METRICS"
        
        # Save to file
        echo "$METRICS" >> "$DATA_FILE"
        SAMPLE_COUNT=$((SAMPLE_COUNT + 1))
        
        # Detect spikes
        if [ "$SAMPLE_COUNT" -gt 1 ] && [ "$PREV_CPU" -gt 0 ] && [ "$CPU" -gt 0 ] && [ "$PREV_CPU" != "0" ]; then
            CPU_INCREASE=$(awk "BEGIN {if ($PREV_CPU == 0) print \"0\"; else printf \"%.1f\", (($CPU - $PREV_CPU) / $PREV_CPU) * 100}")
            CPU_INCREASE_INT=$(echo "$CPU_INCREASE" | cut -d'.' -f1)
            
            if [ "$CPU_INCREASE_INT" -gt "$CPU_SPIKE_THRESHOLD" ] && [ "$IN_CPU_SPIKE" -eq 0 ]; then
                print_spike "CPU spike detected: ${PREV_CPU}m → ${CPU}m (+${CPU_INCREASE}%)"
                IN_CPU_SPIKE=1
                CPU_SPIKE_COUNT=$((CPU_SPIKE_COUNT + 1))
            elif [ "$IN_CPU_SPIKE" -eq 1 ] && [ "$CPU_INCREASE_INT" -lt $((CPU_SPIKE_THRESHOLD / 2)) ]; then
                IN_CPU_SPIKE=0
            fi
        fi
        
        if [ "$SAMPLE_COUNT" -gt 1 ] && [ "$PREV_MEM" -gt 0 ] && [ "$MEM" -gt 0 ] && [ "$PREV_MEM" != "0" ]; then
            MEM_INCREASE=$(awk "BEGIN {if ($PREV_MEM == 0) print \"0\"; else printf \"%.1f\", (($MEM - $PREV_MEM) / $PREV_MEM) * 100}")
            MEM_INCREASE_INT=$(echo "$MEM_INCREASE" | cut -d'.' -f1)
            
            if [ "$MEM_INCREASE_INT" -gt "$MEMORY_SPIKE_THRESHOLD" ] && [ "$IN_MEM_SPIKE" -eq 0 ]; then
                print_spike "Memory spike detected: ${PREV_MEM}Mi → ${MEM}Mi (+${MEM_INCREASE}%)"
                IN_MEM_SPIKE=1
                MEM_SPIKE_COUNT=$((MEM_SPIKE_COUNT + 1))
            elif [ "$IN_MEM_SPIKE" -eq 1 ] && [ "$MEM_INCREASE_INT" -lt $((MEMORY_SPIKE_THRESHOLD / 2)) ]; then
                IN_MEM_SPIKE=0
            fi
        fi
        
        # Display current metrics
        READABLE_TIME=$(date '+%H:%M:%S')
        printf "\r%s - CPU: %4dm | Memory: %4dMi | Samples: %4d | CPU Spikes: %2d | Mem Spikes: %2d" \
            "$READABLE_TIME" "$CPU" "$MEM" "$SAMPLE_COUNT" "$CPU_SPIKE_COUNT" "$MEM_SPIKE_COUNT"
        
        PREV_CPU=$CPU
        PREV_MEM=$MEM
    fi
    
    sleep "$SAMPLE_INTERVAL"
done


#!/bin/bash
# Real-time metrics viewer with ASCII graphs
# Shows live CPU and memory usage with trend visualization

set -e

KUBECONFIG="${KUBECONFIG:-/home/mykastur/gcp_n/install-dir/auth/kubeconfig}"
export KUBECONFIG
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-external-secrets-operator}"

HISTORY_LENGTH=60  # Keep 60 data points
SAMPLE_INTERVAL=2  # Sample every 2 seconds

# Arrays to store history
declare -a CPU_HISTORY
declare -a MEM_HISTORY

# Get operator pod
POD=$(oc get pod -n "$OPERATOR_NAMESPACE" -l app=external-secrets-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POD" ]; then
    echo "Error: Operator pod not found"
    exit 1
fi

# Convert memory to MB
mem_to_mb() {
    local mem=$1
    if [[ $mem =~ ([0-9]+)Mi ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ $mem =~ ([0-9]+)Gi ]]; then
        echo "$((${BASH_REMATCH[1]} * 1024))"
    elif [[ $mem =~ ([0-9]+)Ki ]]; then
        echo "$((${BASH_REMATCH[1]} / 1024))"
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

# Create ASCII bar chart
create_bar() {
    local value=$1
    local max=$2
    local width=50
    
    if [ "$max" -eq 0 ]; then
        max=1
    fi
    
    local bars=$(awk "BEGIN {printf \"%.0f\", ($value / $max) * $width}")
    if [ "$bars" -gt "$width" ]; then
        bars=$width
    fi
    
    printf "["
    for ((i=0; i<$bars; i++)); do
        printf "█"
    done
    for ((i=$bars; i<$width; i++)); do
        printf " "
    done
    printf "]"
}

# Create sparkline
create_sparkline() {
    local -n arr=$1
    local max=$2
    
    if [ "$max" -eq 0 ]; then
        max=1
    fi
    
    local chars=("▁" "▂" "▃" "▄" "▅" "▆" "▇" "█")
    local num_chars=${#chars[@]}
    
    for val in "${arr[@]}"; do
        if [ "$val" -eq 0 ]; then
            printf "${chars[0]}"
        else
            local index=$(awk "BEGIN {printf \"%.0f\", ($val / $max) * ($num_chars - 1)}")
            if [ "$index" -ge "$num_chars" ]; then
                index=$((num_chars - 1))
            fi
            printf "${chars[$index]}"
        fi
    done
}

# Signal handler
cleanup() {
    echo ""
    echo ""
    echo "Monitoring stopped."
    exit 0
}

trap cleanup SIGINT SIGTERM

# Main loop
while true; do
    clear
    
    # Get current metrics
    METRICS=$(oc adm top pod "$POD" -n "$OPERATOR_NAMESPACE" --no-headers 2>/dev/null || echo "N/A N/A")
    CPU=$(echo "$METRICS" | awk '{print $2}')
    MEM=$(echo "$METRICS" | awk '{print $3}')
    
    CPU_M=$(cpu_to_millicores "$CPU")
    MEM_MB=$(mem_to_mb "$MEM")
    
    # Add to history
    CPU_HISTORY+=("$CPU_M")
    MEM_HISTORY+=("$MEM_MB")
    
    # Trim history
    if [ ${#CPU_HISTORY[@]} -gt $HISTORY_LENGTH ]; then
        CPU_HISTORY=("${CPU_HISTORY[@]:1}")
    fi
    if [ ${#MEM_HISTORY[@]} -gt $HISTORY_LENGTH ]; then
        MEM_HISTORY=("${MEM_HISTORY[@]:1}")
    fi
    
    # Calculate statistics
    if [ ${#CPU_HISTORY[@]} -gt 0 ]; then
        CPU_MIN=$(printf '%s\n' "${CPU_HISTORY[@]}" | sort -n | head -1)
        CPU_MAX=$(printf '%s\n' "${CPU_HISTORY[@]}" | sort -n | tail -1)
        CPU_AVG=$(awk "BEGIN {sum=0; for(i=0;i<${#CPU_HISTORY[@]};i++) sum+=${CPU_HISTORY[i]}; printf \"%.0f\", sum/${#CPU_HISTORY[@]}}")
        
        MEM_MIN=$(printf '%s\n' "${MEM_HISTORY[@]}" | sort -n | head -1)
        MEM_MAX=$(printf '%s\n' "${MEM_HISTORY[@]}" | sort -n | tail -1)
        MEM_AVG=$(awk "BEGIN {sum=0; for(i=0;i<${#MEM_HISTORY[@]};i++) sum+=${MEM_HISTORY[i]}; printf \"%.0f\", sum/${#MEM_HISTORY[@]}}")
    else
        CPU_MIN=0
        CPU_MAX=0
        CPU_AVG=0
        MEM_MIN=0
        MEM_MAX=0
        MEM_AVG=0
    fi
    
    # Display dashboard
    echo "╔════════════════════════════════════════════════════════════════════════════╗"
    echo "║          EXTERNAL SECRETS OPERATOR - LIVE METRICS DASHBOARD               ║"
    echo "╚════════════════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "  Pod: $POD"
    echo "  Time: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "  Samples: ${#CPU_HISTORY[@]}/$HISTORY_LENGTH (last $(($HISTORY_LENGTH * $SAMPLE_INTERVAL))s)"
    echo ""
    echo "┌─ CPU USAGE ────────────────────────────────────────────────────────────────┐"
    echo "│"
    echo "│  Current: ${CPU_M}m"
    echo "│  $(create_bar $CPU_M $CPU_MAX) ${CPU_M}m / ${CPU_MAX}m"
    echo "│"
    echo "│  Statistics (last ${#CPU_HISTORY[@]} samples):"
    echo "│    Min: ${CPU_MIN}m  |  Max: ${CPU_MAX}m  |  Avg: ${CPU_AVG}m"
    echo "│"
    echo "│  Trend (${#CPU_HISTORY[@]} samples):"
    echo "│    $(create_sparkline CPU_HISTORY $CPU_MAX)"
    echo "│"
    echo "└────────────────────────────────────────────────────────────────────────────┘"
    echo ""
    echo "┌─ MEMORY USAGE ─────────────────────────────────────────────────────────────┐"
    echo "│"
    echo "│  Current: ${MEM_MB}Mi"
    echo "│  $(create_bar $MEM_MB $MEM_MAX) ${MEM_MB}Mi / ${MEM_MAX}Mi"
    echo "│"
    echo "│  Statistics (last ${#MEM_HISTORY[@]} samples):"
    echo "│    Min: ${MEM_MIN}Mi  |  Max: ${MEM_MAX}Mi  |  Avg: ${MEM_AVG}Mi"
    echo "│"
    echo "│  Trend (${#MEM_HISTORY[@]} samples):"
    echo "│    $(create_sparkline MEM_HISTORY $MEM_MAX)"
    echo "│"
    echo "└────────────────────────────────────────────────────────────────────────────┘"
    echo ""
    
    # Detect spikes
    if [ ${#CPU_HISTORY[@]} -gt 1 ]; then
        PREV_CPU=${CPU_HISTORY[-2]:-0}
        if [ "$PREV_CPU" -gt 0 ]; then
            CPU_CHANGE=$(awk "BEGIN {printf \"%.1f\", (($CPU_M - $PREV_CPU) / $PREV_CPU) * 100}")
            CPU_CHANGE_INT=$(echo "$CPU_CHANGE" | cut -d'.' -f1 | tr -d '-')
            
            if [ "$CPU_CHANGE_INT" -gt 50 ]; then
                echo "  🔥 CPU SPIKE: ${PREV_CPU}m → ${CPU_M}m (+${CPU_CHANGE}%)"
            fi
        fi
        
        PREV_MEM=${MEM_HISTORY[-2]:-0}
        if [ "$PREV_MEM" -gt 0 ]; then
            MEM_CHANGE=$(awk "BEGIN {printf \"%.1f\", (($MEM_MB - $PREV_MEM) / $PREV_MEM) * 100}")
            MEM_CHANGE_INT=$(echo "$MEM_CHANGE" | cut -d'.' -f1 | tr -d '-')
            
            if [ "$MEM_CHANGE_INT" -gt 20 ]; then
                echo "  🔥 MEMORY SPIKE: ${PREV_MEM}Mi → ${MEM_MB}Mi (+${MEM_CHANGE}%)"
            fi
        fi
    fi
    
    echo ""
    echo "  Press Ctrl+C to stop"
    
    sleep $SAMPLE_INTERVAL
done


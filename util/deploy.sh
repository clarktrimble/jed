#!/bin/bash
set -e
cd "$(dirname "$0")"

if [[ -z "$1" || "$1" == "-h" || "$1" == "--help" ]]; then
    cat <<EOF
Usage: $0 <service-name>

Deploy a service to the integration VM.

Expects <service-name>.yaml and <service-name>.env in $(pwd).
EOF
    exit 0
fi

SERVICE="$1"

jed set-svc "${SERVICE}.yaml"
jed set-env "$SERVICE" "${SERVICE}.env"
transship deploy "$SERVICE"

sleep 1
transship tasks "$SERVICE"

TASK=$(transship tasks "$SERVICE" | head -1 | awk '{print $2}')

for i in 1 2 3 4 5; do
    STATE=$(transship tasks "$SERVICE" | grep "$TASK" | awk '{print $3}')
    [[ "$STATE" == "running" ]] && break
    sleep 1
done

if [[ "$STATE" == "running" ]]; then
    echo ""
    echo "logs from $TASK:"
    transship logs "$TASK"
else
    echo ""
    echo "task $TASK not running yet (state: $STATE)"
fi

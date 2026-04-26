#!/bin/bash

set -euo pipefail

usage() {
    echo "Usage: ./run.sh app"
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

case "$1" in
    app)
        docker compose -f docker-compose.app.yaml up --build
        ;;
    *)
        usage
        exit 1
        ;;
esac

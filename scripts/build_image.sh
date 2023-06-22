#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Avalanche root directory
SUBNET_EVM_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Load the versions
source "$SUBNET_EVM_PATH"/scripts/versions.sh

# Load the constants
source "$SUBNET_EVM_PATH"/scripts/constants.sh

echo "Building Docker Image: $DOCKERHUB_REPO:$BUILD_IMAGE_ID"
docker build -t "$DOCKERHUB_REPO:$BUILD_IMAGE_ID" "$SUBNET_EVM_PATH" -f "$SUBNET_EVM_PATH/Dockerfile.container" \
  --build-arg SUBNET_EVM_COMMIT="$SUBNET_EVM_COMMIT" \
  --build-arg CURRENT_BRANCH="$CURRENT_BRANCH"

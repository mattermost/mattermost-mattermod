#!/bin/bash
set -xe

export COMPOSE_PROJECT_NAME=mattermod
export DOCKER_COMPOSE_FILE="$PWD"/docker-compose.yml
export DOCKER_NETWORK="$COMPOSE_PROJECT_NAME"_0

docker-compose -f "$DOCKER_COMPOSE_FILE" down
docker network rm "$DOCKER_NETWORK"

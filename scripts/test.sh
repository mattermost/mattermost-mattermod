#!/bin/bash
set -xe

export COMPOSE_PROJECT_NAME=mattermod
export DOCKER_COMPOSE_FILE="$PWD"/docker-compose.yml
export DOCKER_NETWORK="$COMPOSE_PROJECT_NAME"_0

docker network create "$DOCKER_NETWORK"
docker-compose -f "$DOCKER_COMPOSE_FILE" run -d --rm start_dependencies
docker run --rm --name "${COMPOSE_PROJECT_NAME}_check_mysql" --net "$DOCKER_NETWORK" alpine:latest sh -c "until nc -zv mysql 3306; do echo waiting for mysql; sleep 5; done;"
docker-compose -f "$DOCKER_COMPOSE_FILE" up -d --build
docker-compose logs --tail="all" -t -f
docker-compose -f "$DOCKER_COMPOSE_FILE" down
docker network rm "$DOCKER_NETWORK"

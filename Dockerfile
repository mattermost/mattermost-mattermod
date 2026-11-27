FROM golang:1.27.4@sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666 AS builder

ENV HUB_VERSION 2.14.2
WORKDIR /opt/hub
RUN curl -sSLo hub.tgz https://github.com/github/hub/releases/download/v${HUB_VERSION}/hub-linux-amd64-${HUB_VERSION}.tgz \
    && tar xzf hub.tgz \
    && mv hub-linux-amd64-${HUB_VERSION}/bin/hub .

WORKDIR /go/src/mattermod

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 make build-mattermod

################

FROM ubuntu:noble-20261110.1@sha256:562111b88caa79c71391c0c4d3c2114a7d5c35915c20561fd893cb1e07929e6b

RUN export DEBIAN_FRONTEND="noninteractive" \
    && apt-get update \
    && apt-get upgrade -y \
    && apt-get install --no-install-recommends -y ca-certificates ssh-client git \
    && apt-get clean all \
    && rm -rf /var/cache/apt/

# Ubuntu 24.04 images include an "ubuntu" user with UID/GID 1000; remove it so mattermod can keep UID 1000.
RUN userdel --remove ubuntu \
    && groupadd \
    --gid 1000 mattermod \
    && useradd \
    --home-dir /app \
    --create-home \
    --uid 1000 \
    --gid 1000 \
    --shell /bin/sh \
    --skel /dev/null \
    mattermod \
    && chown -R mattermod:mattermod /app

COPY --from=builder /opt/hub/hub /usr/local/bin/hub
COPY --from=builder /go/src/mattermod/dist/mattermod /usr/local/bin/
COPY --from=builder /go/src/mattermod/hack/cherry-pick.sh /app/scripts/

WORKDIR /app

RUN for d in .ssh repos logs; do \
    mkdir -p /app/${d} ; \
    chown -R mattermod:mattermod /app/${d}/ ; \
    done

USER mattermod
EXPOSE 8080 9000

ENTRYPOINT ["mattermod"]

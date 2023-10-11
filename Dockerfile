FROM golang:1.17.6 AS builder

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

FROM ubuntu:jammy-20230916@sha256:b4b521bfcec90b11d2869e00fe1f2380c21cbfcd799ee35df8bd7ac09e6f63ea

RUN export DEBIAN_FRONTEND="noninteractive" \
    && apt-get update \
    && apt-get upgrade -y \
    && apt-get install --no-install-recommends -y ca-certificates ssh-client git \
    && apt-get clean all \
    && rm -rf /var/cache/apt/

RUN groupadd \
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

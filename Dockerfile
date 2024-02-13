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

FROM ubuntu:noble-20240127.1@sha256:bce129bec07bab56ada102d312ebcfe70463885bdf68fb32182974bd994816e0

RUN export DEBIAN_FRONTEND="noninteractive" \
    && apt-get update \
    && apt-get upgrade -y \
    && apt-get install --no-install-recommends -y ca-certificates ssh-client git \
    && apt-get clean all \
    && rm -rf /var/cache/apt/


COPY --from=builder /opt/hub/hub /usr/local/bin/hub
COPY --from=builder /go/src/mattermod/dist/mattermod /usr/local/bin/
COPY --from=builder /go/src/mattermod/hack/cherry-pick.sh /app/scripts/

WORKDIR /app

RUN chown -R 1000:1000 /app
RUN for d in .ssh repos logs; do \
    mkdir -p /app/${d} ; \
    chown -R 1000:1000 /app/${d}/ ; \
    done

USER 1000
EXPOSE 8080 9000

ENTRYPOINT ["mattermod"]

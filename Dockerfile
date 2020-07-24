FROM golang:1.14.2-alpine AS builder

RUN apk add --update --no-cache ca-certificates bash make gcc musl-dev git openssh wget curl bzr

WORKDIR /go/src/mattermod

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 make build

################

FROM alpine:3.12.0

RUN apk --no-cache add ca-certificates

COPY --from=builder /go/src/mattermod/dist/mattermod /usr/local/bin/
COPY --from=builder /go/src/mattermod/hack/cherry-pick.sh /app/scripts/cherry-pick.sh

WORKDIR /app

RUN mkdir -p /app/logs && chown -R 1000:1000 /app/logs/

USER 1000
EXPOSE 8080

ENTRYPOINT ["mattermod"]

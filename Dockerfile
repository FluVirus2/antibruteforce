# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.5
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod/ \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.0

RUN --mount=type=cache,target=/var/cache/apt \
    --mount=type=cache,target=/var/lib/apt/lists \
    apt-get update && \
    apt-get install --no-install-recommends --assume-yes protobuf-compiler libprotobuf-dev

RUN mkdir -p api/gen && \
    protoc -I=api/proto --go-grpc_out=api/gen --go_out=api/gen api/proto/*/*.proto

RUN --mount=type=cache,target=/go/pkg/mod/ \
    go mod tidy

ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/go/pkg/mod/ \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/server ./cmd/server

FROM alpine:latest AS final

RUN --mount=type=cache,target=/var/cache/apk \
    apk --update add \
        ca-certificates \
        tzdata \
        && \
        update-ca-certificates

ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser
USER appuser

COPY --from=build /bin/server /bin/

EXPOSE 80
EXPOSE 443

ENTRYPOINT [ "/bin/server" ]

FROM golang:1.23.4 AS build
WORKDIR /go/src/app
COPY . .

RUN set -eux; \
    CGO_ENABLED=0 GO111MODULE=on go install .; \
    go run github.com/google/go-licenses@latest save ./... --save_path=/notices;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20241216.0
COPY --from=build /go/bin/poozles /poozles
COPY --from=build /notices /notices
ENTRYPOINT ["/poozles"]

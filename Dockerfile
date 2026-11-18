FROM golang:1.27.3 AS build
WORKDIR /go/src/app
COPY . .

RUN set -eux; \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -trimpath -ldflags=-buildid= -o main .; \
    mkdir /notices; \
    go run github.com/Songmu/gocredits/cmd/gocredits@v1.1.0 . > /notices/CREDITS;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261105.0
COPY --from=build /go/src/app/main /poozles
COPY --from=build /notices /notices
ENTRYPOINT ["/poozles"]

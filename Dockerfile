FROM golang:1.25.5 AS build
WORKDIR /go/src/app
COPY . .

RUN set -eux; \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -trimpath -ldflags=-buildid= -o main .; \
    # modernc.org/mathutil is 3 clause BSD licensed - https://gitlab.com/cznic/mathutil/-/blob/master/LICENSE
    go run github.com/google/go-licenses@latest save ./... --save_path=/notices --ignore modernc.org/mathutil;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20251213.0
COPY --from=build /go/src/app/main /poozles
COPY --from=build /notices /notices
ENTRYPOINT ["/poozles"]

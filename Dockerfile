# 构建阶段：生成内嵌许可证并编译静态二进制。
FROM golang:1.26.5 AS build

ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go run ./tools/licenses \
    && CGO_ENABLED=0 go build -trimpath \
       -ldflags "-w -X github.com/OpenJWC/openjwc_webapi_golang/internal/transport/cli.Version=${VERSION}" \
       -o /out/openjwc ./cmd/openjwc \
    && install -d -m 0700 -o 65532 -g 65532 /out/var/lib/openjwc

# 运行阶段：distroless 静态镜像自带 CA 证书并以 nonroot 运行。
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/openjwc /usr/local/bin/openjwc
COPY --from=build --chown=65532:65532 /out/var/lib /var/lib

ENV OPENJWC_DATA_DIR=/var/lib/openjwc \
    OPENJWC_HTTP_ADDRESS=0.0.0.0:8080

VOLUME ["/var/lib/openjwc"]
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/openjwc"]
CMD ["serve"]

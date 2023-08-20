# syntax = docker/dockerfile:1.3
ARG target_image=golang:1.21
FROM $target_image AS build

WORKDIR /go/src/github.com/mccutchen/go-httpbin

COPY . .

RUN --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    make build buildtests

RUN openssl genrsa -out server.key 2048 && openssl ecparam -genkey -name secp384r1 -out server.key && openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650 -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

FROM gcr.io/distroless/base

COPY --from=build /go/src/github.com/mccutchen/go-httpbin/dist/go-httpbin* /bin/
COPY --from=build /go/src/github.com/mccutchen/go-httpbin/server.key /go/src/github.com/mccutchen/go-httpbin/server.crt /certs/

EXPOSE 8080
ENV HTTPS_CERT_FILE='/certs/server.crt'
ENV HTTPS_KEY_FILE='/certs/server.key'
CMD ["/bin/go-httpbin"]

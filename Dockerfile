# syntax=docker/dockerfile:1.7

FROM golang:1.26 AS build
WORKDIR /src
ENV CGO_ENABLED=0 \
    GO111MODULE=on

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o /out/playlistgen ./cmd/playlistgen

FROM alpine:3.19
RUN adduser -D playlistgen
USER playlistgen
WORKDIR /home/playlistgen
COPY --from=build /out/playlistgen /usr/local/bin/playlistgen

ENTRYPOINT ["/usr/local/bin/playlistgen"]

FROM golang:1.13 as builder

ENV GOFLAGS=-mod=vendor GO111MODULE=on

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o server main.go


FROM ubuntu:bionic

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/server /server
RUN chmod +x /server

RUN useradd -s /sbin/nologin -M -u 10000 -U user
USER user

ENTRYPOINT ["/server"]


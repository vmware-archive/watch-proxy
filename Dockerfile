FROM alpine:3.5
RUN apk add --no-cache ca-certificates
RUN mkdir -p /var/lib/quartermaster
COPY server server
ENTRYPOINT ["/server"]

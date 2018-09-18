FROM alpine:3.5
RUN apk add --no-cache ca-certificates
RUN mkdir -p /var/lib/quartermaster
COPY server server
COPY concord-intermediate.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates
ENTRYPOINT ["/server"]

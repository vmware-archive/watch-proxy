FROM alpine:3.9
RUN apk add --no-cache ca-certificates
COPY server server
ENTRYPOINT ["/server"]

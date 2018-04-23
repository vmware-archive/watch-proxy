FROM golang:1.10.1 AS build

COPY . /go/src/github.com/heptio/quartermaster

WORKDIR /go/src/github.com/heptio/quartermaster
RUN CGO_ENABLED=0 go build -a -ldflags '-s' -installsuffix cgo -o quartermaster && \
chmod +x quartermaster

# copy the binary from the build stage to the final stage
FROM alpine:3.7
COPY --from=build /go/src/github.com/heptio/quartermaster/quartermaster /bin/quartermaster
CMD ["quartermaster"]
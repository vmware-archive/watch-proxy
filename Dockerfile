FROM golang:1.9.2 AS build
ARG GITHUB_TOKEN 
COPY . /go/src/github.com/heptio/clerk

WORKDIR /go/src/github.com/heptio/clerk
RUN git config --global \ 
url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/" && \
    go get -d ./... && \
    CGO_ENABLED=0 go build -a -ldflags '-s' -installsuffix cgo -o app .

# copy the binary from the build stage to the final stage
FROM alpine:3.6
COPY --from=build /go/src/github.com/heptio/clerk/app /clerk
CMD ["/clerk"]
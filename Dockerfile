FROM golang:1.9.2

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install libcurl4-openssl-dev -y && RUN mkdir -p /go
ENV GOPATH=/go

ADD pool /go/src/github.com/umurkontaci/curl/pool
ADD transport /go/src/github.com/umurkontaci/curl/transport

RUN go get github.com/umurkontaci/curl/transport
RUN go get github.com/umurkontaci/curl/pool
CMD ["go", "test", "-race", "github.com/umurkontaci/curl/transport", "github.com/umurkontaci/curl/pool"]
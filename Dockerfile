FROM golang:1.9.2

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install libcurl4-openssl-dev -y && mkdir -p /go
ENV GOPATH=/go

ADD pool /go/src/github.com/umurkontaci/go-curl-transport/pool
ADD transport /go/src/github.com/umurkontaci/go-curl-transport/transport

WORKDIR /go/src/github.com/umurkontaci/go-curl-transport

RUN cd pool && go get -x -v
RUN cd transport && go get -x -v
CMD ["go", "test", "-v", "github.com/umurkontaci/go-curl-transport/transport", "github.com/umurkontaci/go-curl-transport/pool"]

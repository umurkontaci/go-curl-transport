FROM golang:1.9.2

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install libcurl4-openssl-dev -y && mkdir -p /go
ENV GOPATH=/go

ADD pool /go/src/github.com/umurkontaci/curl/pool
ADD transport /go/src/github.com/umurkontaci/curl/transport

WORKDIR /go/src/github.com/umurkontaci/curl

RUN cd pool && go get -x -v
RUN cd transport && go get -x -v
CMD ["go", "test", "-v", "github.com/umurkontaci/curl/transport", "github.com/umurkontaci/curl/pool"]

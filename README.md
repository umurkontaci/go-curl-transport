# Curl Transport

[![CircleCI](https://circleci.com/gh/umurkontaci/go-curl-transport.svg?style=svg)](https://circleci.com/gh/umurkontaci/go-curl-transport)

Curl Transport is an open source software that allows you to use Go's native `http.Client` backed with curl.

It's a drop in replacement for `http.Client`'s default transport.

The interface remains the same but `http.Client` would use libcurl to make the call.


## Getting Started

### Prerequisites

- libcurl headers (version >=7.10.0)
  - *Ubuntu*: `sudo apt-get install libcurl4-openssl-dev`
  - *OSX*: `brew install curl`
- Go (version >=1.7) (1.6 could work, not tested)


### Get the package
```
go get -u github.com/umurkontaci/go-curl-transport/transport
```

### Try it out
```go
package main

import (
  "net/http"
  "io/ioutil"
  "log"

  "github.com/umurkontaci/go-curl-transport/transport"
)

func main() {
    cli := http.Client{Transport: transport.New()}
    res, err := cli.Get("https://httpbin.org/uuid")
    if err != nil {
        panic(err)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }

    log.Print(body)
}
```


## Why?

curl is an extremely mature piece of software that's well integrated
with different systems. Most problems you can encounter with http clients
are already solved problems in curl.

Putting this behind `http.Client`'s transport allows you to keep the same,
well-defined idiomatic go interface, but have all the power of curl.

For example, curl in OSX integrates with OSX's Keychain so any certificate
that's trusted at the OS level will be trusted. No more concatanating certificates
to a file.


Or if you are working with Kerberos or GSS, since curl integrates with Kerberos and GSS libraries really well,
all you have to do is to set the flag and you have a GSSNEGOTIATE capable http Client.

```go
  func GssClient() (*http.Client, error) {
      t := transport.New()
      t.PostConfigure = func(c *curl.CURL, r *http.Request) error {
          err := c.Setopt(curl.OPT_HTTPAUTH, curl.AUTH_GSSNEGOTIATE)
          if err != nil {
              return nil, err
          }
          err = c.Setopt(curl.OPT_USERPWD, ":")
          if err != nil {
              return nil, err
          }
     }
     return http.Client{Transport: t}, nil
  }
```


## Caveats

- HTTP/2 is not supported (yet)
- This library is very new, so don't use it in production.


// transport exports a Transport struct that will make http.Client to use libcurl when making requests.
// Transport supports HTTP/1.0 and HTTP/1.1. It does not support HTTP/2 at this moment.
// This package requires libcurl headers to be available in the system.
package transport

import (
	"net/http"
	"bytes"
	"github.com/umurkontaci/go-curl"
	"bufio"
	"os"
	"strings"
	"fmt"
	"errors"
	"sync"
	"io"
	"runtime"
	"github.com/umurkontaci/curl/pool"
)

func init() {
	curl.GlobalInit(curl.GLOBAL_ALL)
}

// PostConfigure is called after the transport initializes curl and sets the proper options.
// This your chance set up additional configuration options to curl as you see fit.
// For example, if you want curl to do automatic Kerberos authentication using GSS API, you would do:
//  func GssClient() (*http.Client, error) {
//      t := transport.New()
//      t.PostConfigure = func(c *curl.CURL, r *http.Request) error {
//          err := c.Setopt(curl.OPT_HTTPAUTH, curl.AUTH_GSSNEGOTIATE)
//          if err != nil {
//              return nil, err
//          }
//          err = c.Setopt(curl2.OPT_USERPWD, ":")
//          if err != nil {
//              return nil, err
//          }
//      }
//      return http.Client{&t}, nil
//  }
//
// When working with curl directly, keep in mind that you are working with CGO and need to take the appropriate measures
// for garbage collection.
type PostConfigure func(c *curl.CURL, r *http.Request) error

// Transport is a replacement http.Client.Transport that will make http.Client to use curl as transport while keeping
// the rest of the interface the same.
// Example:
//  c := http.Client{&transport.New()}
//  res, _ := c.Get("https://httpbin.org/uuid")
//  log.Printf(ioutil.ReadAll(res.Body))
// Doing this will make sure http.Client uses curl as the transport. You don't need to change anything or the way
// you use http.Client.
type Transport struct {
	Pool *pool.FinalizingPool // The FinalizingPool to store curl instances. You should not reassign or copy this value.
	PostConfigure
}

// New initializes a new Transport with a FinalizingPool
func New() Transport {
	return Transport{Pool: &pool.FinalizingPool{}}
}

func configure(c *curl.CURL, req *http.Request) error {
	options := make(map[int]interface{})
	options[curl.OPT_URL] = req.URL.String()
	options[curl.OPT_COOKIESESSION] = true
	options[curl.OPT_VERBOSE] = os.Getenv("CURL_DEBUG") == "1"
	options[curl.OPT_MAXREDIRS] = 20
	if req.Method != "GET" && req.Method != "POST" {
		options[curl.OPT_CUSTOMREQUEST] = strings.ToUpper(req.Method)
	}

	if req.Method == "POST" {
		options[curl.OPT_POST] = true
	}

	options[curl.OPT_TCP_NODELAY] = true
	if req.ProtoMajor > 1 {
		return errors.New("only HTTP/1.x is supported")
	}

	switch req.ProtoMinor {
	case 0:
		options[curl.OPT_HTTP_VERSION] = curl.HTTP_VERSION_1_0
	case 1:
		options[curl.OPT_HTTP_VERSION] = curl.HTTP_VERSION_1_1
	default:
		return errors.New("unknown minor http version")
	}

	for key, value := range options {
		if err := c.Setopt(key, value); err != nil {
			return err
		}
	}
	return nil
}

func postConfigure(_ *curl.CURL, _ *http.Request) error {
	return nil
}

type WriterResetter interface {
	io.Writer
	Resetter
}

// Resetter resets a Writer to clear everything in their buffer and start over.
type Resetter interface {
	Reset()
}

func readHeader(responseHeaders []byte, header WriterResetter, body WriterResetter) error {
	reader := bytes.NewReader(responseHeaders)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// curl will send multiple responses to the same stream and this will confuse the HttpResponseReader.
		// For example curl can receive HTTP/1.1 100 Continue and send the payload. In this case, curl will send the 100
		// response to the stream and then send the final response to the stream. HttpResponseReader is confused by
		// this behaviour and it only accepts a single HTTP response. To solve this, if the header we receive is a start
		// of a HTTP response, we reset the buffers.
		if strings.HasPrefix(line, "HTTP/1.") {
			header.Reset()
			body.Reset()
		}
		_, err := io.WriteString(header, line)
		if err != nil {
			return err
		}
		_, err = header.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}
	return nil
}

func readBody(respBody []byte, body io.Writer) (err error) {
	_, err = body.Write(respBody)
	return
}

func writePayload(buf []byte, payload io.Reader) (int, error) {
	return payload.Read(buf)
}

func (t *Transport) pool() (*pool.FinalizingPool, error) {
	if t.Pool == nil {
		return nil, errors.New("transport FinalizingPool is not initialized")
	}
	return t.Pool, nil
}

func (t *Transport) getPostConfigure() PostConfigure {
	if t.PostConfigure != nil {
		return t.PostConfigure
	}
	return postConfigure
}

// RoundTrip is the backbone of Transport, it initializes the curl instance, sets the configuration, sends the request
// receives and parses the response and returns a http.Response.
func (t Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()

	curlPool, err := t.pool()
	if err != nil {
		return nil, err
	}
	easy := curlPool.Get()

	err = configure(easy, req)
	if err != nil {
		return nil, err
	}

	var callbackErr error
	defer curlPool.Put(easy)
	defer easy.Reset()

	headers := make([]string, len(req.Header)+1)

	for key, values := range req.Header {
		headers = append(headers, fmt.Sprintf("%s: %s", key, strings.Join(values, ",")))
	}
	if req.ContentLength > 0 {
		headers = append(headers, fmt.Sprintf("Content-Length: %d", req.ContentLength))
	}

	err = easy.Setopt(curl.OPT_HTTPHEADER, headers)
	if err != nil {
		return nil, err
	}
	readFunction := func(ptr []byte, _ interface{}) int {
		n, err := writePayload(ptr, req.Body)

		if err != nil && n > 0 {
			callbackErr = err
			return n
		}
		return n
	}

	if req.Body != nil {
		easy.Setopt(curl.OPT_READFUNCTION, readFunction)
	}

	bodyBuffer := bytes.Buffer{}
	bodyMutex := sync.Mutex{}
	writeFunction := func(response []byte, _ interface{}) bool {
		bodyMutex.Lock()
		defer bodyMutex.Unlock()
		err := readBody(response, &bodyBuffer)

		if err != nil {
			callbackErr = err
		}
		return true
	}
	easy.Setopt(curl.OPT_WRITEFUNCTION, writeFunction)

	headerBuffer := bytes.Buffer{}
	headerFunction := func(responseHeaders []byte, _ interface{}) bool {
		bodyMutex.Lock()
		defer bodyMutex.Unlock()

		err := readHeader(responseHeaders, &headerBuffer, &bodyBuffer)

		if err != nil {
			callbackErr = err
		}
		return true
	}
	easy.Setopt(curl.OPT_HEADERFUNCTION, headerFunction)
	t.getPostConfigure()(easy, req)
	err = easy.Perform()

	// Mark these available after easy.Perform(), Go has no idea if / when these functions will be used in C and
	// they might can garbage collected and SEGFAULT in C.
	runtime.KeepAlive(readFunction)
	runtime.KeepAlive(writeFunction)
	runtime.KeepAlive(headerFunction)

	if err != nil {
		return nil, err
	}

	if callbackErr != nil {
		return nil, callbackErr
	}

	headerBuffer.ReadFrom(&bodyBuffer)
	res, err = http.ReadResponse(bufio.NewReader(&headerBuffer), req)

	return res, err
}

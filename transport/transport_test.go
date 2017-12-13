package transport

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, called chan bool, method string, body []byte, expectedPayload []byte) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			t.Errorf("Expected method to be '%s', got '%s'", method, r.Method)
		}
		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if expectedPayload != nil && !bytes.Equal(payload, expectedPayload) {
			t.Errorf(`
Server did not receive expected payload:
Expected: %s
Got: %s
`, expectedPayload, payload)
		}
		w.Write(body)
		called <- true
		close(called)
	}))
	return ts
}

func expect(t *testing.T, called chan bool, expectedResponse []byte, resp *http.Response) {
	select {
	case v := <-called:
		if !v {
			t.Errorf("Server did not receive any request")
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		if expectedResponse != nil && !bytes.Equal(b, expectedResponse) {
			t.Errorf(`
Client did not receive the expected body:
Expected: %s
Got: %s
`, expectedResponse, b)
		}

	case <-time.After(time.Second * 1):
		t.Errorf("Timed out while waiting for a request")
	}
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestTransport_RoundTrip_ShouldErrorWhenPoolIsNotDefined(t *testing.T) {
	client := http.Client{Transport: Transport{}}
	_, err := client.Get("http://httpbin.org")
	if err == nil {
		t.Errorf("Expected to return error")
	}
}

func TestMakesCalls(t *testing.T) {
	called := make(chan bool, 1)
	ts := serve(t, called, "GET", []byte{}, nil)
	defer ts.Close()
	client := http.Client{Transport: New()}
	r, err := client.Get(ts.URL)
	checkErr(t, err)
	expect(t, called, nil, r)
}

func TestSetsMethod(t *testing.T) {
	called := make(chan bool, 1)
	ts := serve(t, called, "PUT", []byte{}, nil)
	defer ts.Close()
	client := http.Client{Transport: New()}
	req, err := http.NewRequest("PUT", ts.URL, nil)

	checkErr(t, err)
	res, err := client.Do(req)

	checkErr(t, err)
	expect(t, called, nil, res)
}

func TestSendsBody(t *testing.T) {
	called := make(chan bool, 1)
	respBody := []byte("cool")
	payload := []byte("hello world")
	ts := serve(t, called, "POST", respBody, payload)
	defer ts.Close()
	client := http.Client{Transport: New()}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(payload))
	checkErr(t, err)

	res, err := client.Do(req)

	checkErr(t, err)
	expect(t, called, respBody, res)
}

func TestTransport_WrongUrl(t *testing.T) {
	client := http.Client{Transport: New()}
	_, err := client.Get("http://localhost:34004")
	if err == nil {
		t.Errorf("Expected an error")
	}
	if !strings.Contains(err.Error(), "Couldn't connect to server") {
		t.Errorf("%v", err)
	}
}

func TestReceivesGetBody(t *testing.T) {
	called := make(chan bool, 1)
	respBody := []byte("nice")
	ts := serve(t, called, "GET", respBody, nil)
	defer ts.Close()

	client := http.Client{Transport: New()}
	res, err := client.Get(ts.URL)

	checkErr(t, err)
	expect(t, called, respBody, res)
}

func TestReceivesBodyAfterExpect100ContinueHeaders(t *testing.T) {
	called := make(chan bool, 1)
	respBody := []byte("cool")
	payload := []byte("hello string")
	ts := serve(t, called, "POST", respBody, payload)

	defer ts.Close()
	client := http.Client{Transport: New()}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewReader(payload))
	checkErr(t, err)
	res, err := client.Do(req)
	checkErr(t, err)
	expect(t, called, respBody, res)
}

func TestTransport_RoundTrip_Get_GoRoutine(t *testing.T) {

	respBody := []byte("nice")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(respBody)
	}))
	defer ts.Close()
	testCount := 4000
	c := make(chan bool, testCount)
	client := http.Client{Transport: New()}
	for i := 0; i < i; i++ {
		time.Sleep(time.Microsecond)
		go func() {
			_, err := client.Get(ts.URL)
			checkErr(t, err)
			c <- true
		}()
	}
	for i := 0; i < i; i++ {
		select {
		case <-c:
		case <-time.After(time.Second * 5):
			t.Errorf("Timeout after 2 seconds")
		}
	}
}

func TestTransport_RoundTrip_AssignsPool(t *testing.T) {
	called := make(chan bool, 1)
	ts := serve(t, called, "GET", []byte{}, nil)
	tr := New()
	defer ts.Close()
	client := http.Client{Transport: tr}
	r, err := client.Get(ts.URL)
	checkErr(t, err)
	expect(t, called, nil, r)
	if tr.Pool == nil {
		t.Errorf("Expected Transport.FinalizingPool not to be nil")
	}
}

func TestTransport_RoundTrip_HTTP2(t *testing.T) {
	tr := New()
	cli := http.Client{Transport: tr}
	req := http.Request{RequestURI: "http://localhost:10000", ProtoMajor: 2}
	_, err := cli.Do(&req)
	if err == nil {
		t.Error("Expected an error for HTTP/2")
	}
}

func BenchmarkNetHttpGet(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer ts.Close()
	cli := http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cli.Get(ts.URL)
	}
	b.ReportAllocs()
}

func BenchmarkNetCurlGet(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer ts.Close()
	cli := http.Client{Transport: New()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cli.Get(ts.URL)
	}
	b.ReportAllocs()
}

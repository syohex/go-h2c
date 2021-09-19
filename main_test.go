package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_readInputNoBody(t *testing.T) {
	input := strings.NewReader(`HEAD / HTTP/1.1
Host: curl.haxx.se
User-Agent: moo
Shoesize: 12
`)
	req, err := readInput(input)
	assert.Nil(t, err)

	assert.Equal(t, req.method, "HEAD", "method in request line")
	assert.Equal(t, req.path, "/", "path in request line")
	assert.Equal(t, req.http, "HTTP/1.1", "http version in request line")

	headers := [][]string{
		{"host", "curl.haxx.se"},
		{"user-agent", "moo"},
		{"shoesize", "12"},
	}
	for _, header := range headers {
		assert.Equal(t, req.header[header[0]], header[1], header[0]+" header")
	}

	input2 := strings.NewReader(`HEAD / HTTP/1.1
User-Agent: moo
Shoesize: 12
`)
	_, err2 := readInput(input2)
	assert.NotNil(t, err2, "no host header")

	input3 := strings.NewReader(`FOO / HTTP/1.1`)
	_, err3 := readInput(input3)
	assert.NotNil(t, err3, "unsupported HTTP method")
}

func Test_readInputWithBody(t *testing.T) {
	input := strings.NewReader(`PUT /this is me HTTP/2
Host: curl.haxx.se
User-Agent: moo on you all
Shoesize: 12
Cookie: a=12; b=23
Content-Type: application/json
Content-Length: 57

{"I do not speak": "jason"}
{"I do not write": "either"}`)
	req, err := readInput(input)
	assert.Nil(t, err)

	expected := []string{`{"I do not speak": "jason"}`, `{"I do not write": "either"}`}
	assert.Equal(t, req.body, expected, "body")
}

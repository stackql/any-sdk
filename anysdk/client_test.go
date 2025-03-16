package anysdk

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/assert"
)

func nop() {
	fmt.Println("nop")
}

func TestClient(t *testing.T) {
	res := &http.Response{
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"a": { "b": [ "c" ] } }`)),
	}
	s := NewStringSchema(nil, "", "")
	pr, err := s.ProcessHttpResponseTesting(res, "", "", "")
	assert.NilError(t, err)
	assert.Assert(t, pr != nil)
}

package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRelativeURLPreservesEncodedPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	}))
	defer server.Close()
	for _, test := range []struct {
		base string
		path string
		want string
	}{
		{base: "/v1", path: "/files/a%2Fb", want: "/v1/files/a%2Fb"},
		{base: "/tenant%2Fone/", path: "/files/a%2fb", want: "/tenant%2Fone/files/a%2fb"},
		{base: "/v1", path: "/files/%E4%B8%AD%20%E6%96%87", want: "/v1/files/%E4%B8%AD%20%E6%96%87"},
		{base: "/v1?token=base", path: "/files/a%252Fb?limit=2#part", want: "/v1/files/a%252Fb?limit=2&token=base"},
	} {
		t.Run(test.path, func(t *testing.T) {
			client := New(Config{BaseURL: server.URL + test.base})
			response, err := client.Get(test.path)
			if err != nil {
				t.Fatal(err)
			}
			if string(response.Body) != test.want {
				t.Fatalf("upstream request URI=%q, want %q", response.Body, test.want)
			}
		})
	}
}

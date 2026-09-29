package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/validation"
)

func TestResolve(t *testing.T) {
	configured := Policy{Default: 25, Max: 60}
	cases := map[string]struct {
		policy  Policy
		request Request
		want    Page
	}{
		"zero policy defaults":         {Policy{}, Request{}, Page{Number: 1, Size: 20}},
		"configured default":           {configured, Request{}, Page{Number: 1, Size: 25}},
		"page size capped at max":      {configured, Request{Page: 2, PageSize: 100}, Page{Number: 2, Size: 60, Offset: 60}},
		"limit wins over page size":    {configured, Request{PageSize: 10, Limit: 30}, Page{Number: 1, Size: 30}},
		"limit capped at max":          {configured, Request{Page: 2, Limit: 80}, Page{Number: 2, Size: 60, Offset: 60}},
		"explicit offset wins":         {configured, Request{Page: 3, PageSize: 10, Offset: 5}, Page{Number: 3, Size: 10, Offset: 5}},
		"list all is unbounded":        {configured, Request{Page: 2, PageSize: 10, ListAll: true}, Page{Number: 2}},
		"default above max is clamped": {Policy{Default: 500, Max: 50}, Request{}, Page{Number: 1, Size: 50}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.policy.Resolve(tc.request); got != tc.want {
				t.Fatalf("Resolve(%+v) = %+v, want %+v", tc.request, got, tc.want)
			}
		})
	}
}

func TestNewMeta(t *testing.T) {
	if got := NewMeta(41, Page{Number: 2, Size: 20}); got != (Meta{Total: 41, Page: 2, PageSize: 20, TotalPages: 3}) {
		t.Fatalf("meta = %+v", got)
	}
	if got := NewMeta(41, Page{Number: 1}); got.TotalPages != 0 || got.PageSize != 0 {
		t.Fatalf("list-all meta = %+v, want zero page size and pages", got)
	}
}

// The size cap is api.max_per_page, so binding must not reject what the policy
// would allow; a fixed max=100 tag once returned 400 for page_size=150 while
// limit=150 went through.
func TestRequestLeavesTheSizeCapToThePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?page_size=500", nil)

	var request Request
	if err := validation.BindQuery(c, &request); err != nil {
		t.Fatalf("page_size above the default cap was rejected: %v", err)
	}
	if got := (Policy{Max: 200}).Resolve(request); got.Size != 200 {
		t.Fatalf("page size = %d, want the policy cap 200", got.Size)
	}
}

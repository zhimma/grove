package validation

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhimma/grove/pkg/errx"
)

func bindNestedJSON(body string, target any) error {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	return BindJSONStrict(c, target)
}

func fieldErrors(t *testing.T, err error) map[string][]string {
	t.Helper()
	httpErr := errx.Normalize(err)
	if httpErr == nil {
		t.Fatal("expected validation error")
	}
	fields, ok := httpErr.Data["errors"].(map[string][]string)
	if !ok {
		t.Fatalf("unexpected errors: %#v", httpErr.Data)
	}
	return fields
}

func TestNestedValidationPreservesPathsIndexesAndLabels(t *testing.T) {
	type item struct {
		DisplayName string `json:"display_name" binding:"required" label:"显示名"`
	}
	var input struct {
		Items      []item          `json:"items" binding:"dive"`
		Primary    *item           `json:"primary"`
		Secondary  item            `json:"secondary"`
		Dictionary map[string]item `json:"dictionary" binding:"dive"`
	}
	err := bindNestedJSON(`{"items":[{},{}],"primary":{},"secondary":{},"dictionary":{"with.dot":{}}}`, &input)
	want := map[string][]string{
		"items[0].display_name":             {"显示名不能为空"},
		"items[1].display_name":             {"显示名不能为空"},
		"primary.display_name":              {"显示名不能为空"},
		"secondary.display_name":            {"显示名不能为空"},
		"dictionary[with.dot].display_name": {"显示名不能为空"},
	}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

func TestNestedValidationFlattensOnlyUntaggedEmbeddedFields(t *testing.T) {
	type Embedded struct {
		Code string `json:"code" binding:"required" label:"编码"`
	}
	type input struct {
		Embedded
		Tagged Embedded `json:"tagged"`
	}
	var target input
	err := bindNestedJSON(`{"tagged":{}}`, &target)
	want := map[string][]string{"code": {"编码不能为空"}, "tagged.code": {"编码不能为空"}}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

func TestValidationFieldNamesFollowTheBindingSource(t *testing.T) {
	type input struct {
		Name string `json:"json_name" form:"query_name" uri:"uri_name" binding:"required" label:"名称"`
	}
	for _, test := range []struct {
		key  string
		bind func(*gin.Context, any) error
	}{
		{key: "json_name", bind: BindJSON},
		{key: "query_name", bind: BindQuery},
		{key: "uri_name", bind: BindURI},
	} {
		t.Run(test.key, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
			c.Params = gin.Params{{Key: "uri_name", Value: ""}}
			var target input
			if got := fieldErrors(t, test.bind(c, &target)); !reflect.DeepEqual(got, map[string][]string{test.key: {"名称不能为空"}}) {
				t.Fatalf("errors=%#v", got)
			}
		})
	}
}

func TestNestedValidationResolvesSiblingLabels(t *testing.T) {
	var input struct {
		Account struct {
			Password     string `json:"password" label:"密码"`
			Confirmation string `json:"confirmation" binding:"eqfield=Password" label:"确认密码"`
		} `json:"account"`
	}
	err := bindNestedJSON(`{"account":{"password":"one","confirmation":"two"}}`, &input)
	want := map[string][]string{"account.confirmation": {"确认密码必须与密码一致"}}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

func TestNestedJSONTypeErrorPreservesParentPath(t *testing.T) {
	var input struct {
		Person struct {
			Age int `json:"age" label:"年龄"`
		} `json:"person"`
	}
	err := bindNestedJSON(`{"person":{"age":"invalid"}}`, &input)
	want := map[string][]string{"person.age": {"年龄格式不正确"}}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

func TestJSONTypeErrorInCollectionPreservesFieldLabel(t *testing.T) {
	var input struct {
		People []struct {
			Age int `json:"age" label:"年龄"`
		} `json:"people"`
	}
	err := bindNestedJSON(`{"people":[{"age":"invalid"}]}`, &input)
	want := map[string][]string{"people[0].age": {"年龄格式不正确"}}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

func TestJSONTypeErrorInMapPreservesFieldLabel(t *testing.T) {
	var input struct {
		People map[string]struct {
			Age int `json:"age" label:"年龄"`
		} `json:"people"`
	}
	err := bindNestedJSON(`{"people":{"east":{"age":"invalid"}}}`, &input)
	want := map[string][]string{"people[east].age": {"年龄格式不正确"}}
	if got := fieldErrors(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors=%#v, want %#v", got, want)
	}
}

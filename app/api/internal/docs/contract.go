package docs

import (
	"net/http"

	"github.com/zhimma/grove/app/api/handler"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/docsui"
)

func spec(cfg *config.Config) docsui.Document {
	doc := docsui.NewDocument(
		cfg.Docs.Title,
		cfg.Docs.Description,
		cfg.Docs.Version,
		resolveAPIBasePath(cfg),
	)
	if !demoEnabled(cfg) {
		return doc
	}

	issueTokenRequest := doc.AddSchema("IssueAccessTokenRequest", handler.IssueAccessTokenRequest{})
	issueTokenResponse := doc.AddSchema("IssueAccessTokenResponse", handler.IssueAccessTokenResponse{})
	pingResponse := doc.AddSchema("PingResponse", handler.PingResponse{})
	profileResponse := doc.AddSchema("ProfileResponse", handler.ProfileResponse{})
	dispatchEchoRequest := doc.AddSchema("DispatchEchoJobRequest", handler.DispatchEchoJobRequest{})
	dispatchEchoResponse := doc.AddSchema("DispatchEchoJobResponse", handler.DispatchEchoJobResponse{})

	addOperation(&doc, "/ping", http.MethodGet, "ping", "公共连通性检查", false,
		docsui.ParametersFor(handler.PingRequest{}, "form", "query"), nil, pingResponse)
	addOperation(&doc, "/auth/access-token", http.MethodPost, "issueAccessToken", "签发示例访问令牌", false,
		nil, docsui.JSONBody("令牌请求", issueTokenRequest, true), issueTokenResponse)
	addOperation(&doc, "/profile", http.MethodGet, "getProfile", "获取当前示例用户", true,
		nil, nil, profileResponse)
	addOperation(&doc, "/jobs/echo", http.MethodPost, "dispatchEchoJob", "投递示例异步任务", true,
		nil, docsui.JSONBody("任务请求", dispatchEchoRequest, true), dispatchEchoResponse)
	return doc
}

func addOperation(
	doc *docsui.Document,
	path string,
	method string,
	id string,
	summary string,
	bearer bool,
	parameters []docsui.Parameter,
	body *docsui.RequestBody,
	response docsui.Schema,
) {
	doc.Add(path, docsui.Operation{
		Method:      method,
		ID:          id,
		Summary:     summary,
		BearerAuth:  bearer,
		Parameters:  parameters,
		RequestBody: body,
		Responses:   docsui.StandardResponses("请求成功", response),
	})
}

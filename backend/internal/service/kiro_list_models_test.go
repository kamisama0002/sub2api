//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestKiroListAvailableModelsPaginatesAndUsesProfileArn(t *testing.T) {
	account := &Account{
		ID:       7,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"api_region": "us-east-1",
		},
	}
	var requests []*http.Request
	do := func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		body := `{"models":[{"modelId":"claude-opus-5.5"}],"nextToken":""}`
		if len(requests) == 1 {
			body = `{"models":[{"modelId":"claude-opus-5.5"},{"modelId":"gpt-5.6-sol"}],"nextToken":"page-2"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	ids, err := kiroListAvailableModelIDsWithDo(context.Background(), account, "token", do)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-opus-5.5", "gpt-5.6-sol"}, ids)
	require.Len(t, requests, 2)

	first := requests[0].URL
	require.Equal(t, http.MethodGet, requests[0].Method)
	require.Equal(t, "q.us-east-1.amazonaws.com", first.Host)
	require.Equal(t, "/ListAvailableModels", first.Path)
	require.Equal(t, "AI_EDITOR", first.Query().Get("origin"))
	require.Equal(t, "50", first.Query().Get("maxResults"))
	require.Equal(t, kiroBuilderIDProfileARN, first.Query().Get("profileArn"))
	require.Empty(t, first.Query().Get("nextToken"))
	require.Equal(t, "Bearer token", requests[0].Header.Get("Authorization"))
	require.Equal(t, "application/json", requests[0].Header.Get("Accept"))
	require.Empty(t, requests[0].Header.Get("X-Amz-Target"))
	require.Equal(t, "page-2", requests[1].URL.Query().Get("nextToken"))
	require.Equal(t, []string{"claude-opus-5-5", "claude-opus-5-5-thinking", "gpt-5.6-sol"}, kiroSyncPublicModelNames(ids))
	_, metadata := kiroPublicCatalog(ids)
	require.Equal(t, "claude-opus-5.5", metadata["claude-opus-5-5"].ID)
	require.Equal(t, "claude-opus-5.5", metadata["claude-opus-5-5-thinking"].ID)
	require.Equal(t, "gpt-5.6-sol", metadata["gpt-5.6-sol"].ID)
}

func TestKiroListAvailableModelsRetriesForbiddenRegion(t *testing.T) {
	account := &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"api_region": "us-east-1",
		},
	}
	var hosts []string
	do := func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		if req.URL.Host == "q.us-east-1.amazonaws.com" {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(`{"message":"forbidden"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"models":[{"modelId":"claude-sonnet-5"}]}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	ids, err := kiroListAvailableModelIDsWithDo(context.Background(), account, "token", do)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-5"}, ids)
	require.Equal(t, []string{"q.us-east-1.amazonaws.com", "q.eu-central-1.amazonaws.com"}, hosts)
}

func TestKiroListAvailableModelsAPIKeyOmitsProfileArn(t *testing.T) {
	account := &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "ksk_test",
		},
	}
	do := func(req *http.Request) (*http.Response, error) {
		require.Empty(t, req.URL.Query().Get("profileArn"))
		require.Equal(t, []string{"API_KEY"}, req.Header["TokenType"])
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"models":[{"modelId":"claude-opus-5"}]}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	ids, err := kiroListAvailableModelIDsWithDo(context.Background(), account, "ksk_test", do)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-opus-5"}, ids)
	require.Equal(t, []string{"claude-opus-5", "claude-opus-5-thinking"}, kiroSyncPublicModelNames(ids))
}

func TestKiroListAvailableModelsStopsAfterTenPages(t *testing.T) {
	account := &Account{Platform: PlatformKiro, Type: AccountTypeOAuth}
	calls := 0
	do := func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"models":[{"modelId":"claude-opus-5"}],"nextToken":"more"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	ids, err := kiroListAvailableModelIDsWithDo(context.Background(), account, "token", do)
	require.NoError(t, err)
	require.Equal(t, kiroListModelsMaxPages, calls)
	require.Equal(t, []string{"claude-opus-5"}, ids)
}

func TestBuildKiroRelayModelsRequestUsesBaseURL(t *testing.T) {
	svc := &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{
		URLAllowlist: config.URLAllowlistConfig{Enabled: false},
	}}}
	account := &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "relay-key",
			"base_url": "https://relay.example.com/v1",
		},
	}

	req, err := svc.buildKiroRelayModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/v1/models", req.URL.String())
	require.Equal(t, "relay-key", req.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
}

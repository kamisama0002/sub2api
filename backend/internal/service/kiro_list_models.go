package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/google/uuid"
)

const (
	kiroListModelsMaxPages       = 10
	kiroListModelsPageSize       = "50"
	kiroListModelsFallbackRegion = "eu-central-1"
	kiroListModelsOrigin         = "AI_EDITOR"
)

type kiroModelsDoFunc func(req *http.Request) (*http.Response, error)

type kiroListAvailableModelsResponse struct {
	Models []struct {
		ModelID string `json:"modelId"`
	} `json:"models"`
	NextToken string `json:"nextToken"`
}

// kiroListAvailableModelIDs 调用 GET https://q.{region}.amazonaws.com/ListAvailableModels。
// 账号区域返回 403 时再试 eu-central-1。最多翻 10 页。
func kiroListAvailableModelIDs(ctx context.Context, account *Account, token string) ([]string, error) {
	return kiroListAvailableModelIDsWithDo(ctx, account, token, kiroModelsHTTPDo(account))
}

func kiroModelsHTTPDo(account *Account) kiroModelsDoFunc {
	return func(req *http.Request) (*http.Response, error) {
		client, err := httpclient.GetClient(httpclient.Options{
			ProxyURL:           kiroProxyURL(account),
			Timeout:            30 * time.Second,
			ValidateResolvedIP: true,
		})
		if err != nil {
			return nil, err
		}
		return client.Do(req)
	}
}

func kiroListAvailableModelIDsWithDo(ctx context.Context, account *Account, token string, do kiroModelsDoFunc) ([]string, error) {
	if account == nil {
		return nil, fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("kiro access token is empty")
	}
	if do == nil {
		return nil, fmt.Errorf("kiro models transport is nil")
	}

	region := kiroAPIRegion(account)
	ids, status, err := kiroListModelsPages(ctx, account, token, region, do)
	if status == http.StatusForbidden && !strings.EqualFold(region, kiroListModelsFallbackRegion) {
		fallbackIDs, _, fallbackErr := kiroListModelsPages(ctx, account, token, kiroListModelsFallbackRegion, do)
		if fallbackErr == nil {
			return fallbackIDs, nil
		}
		return nil, fallbackErr
	}
	return ids, err
}

func kiroListModelsPages(ctx context.Context, account *Account, token, region string, do kiroModelsDoFunc) ([]string, int, error) {
	seen := make(map[string]struct{})
	var ids []string
	nextToken := ""
	for page := 0; page < kiroListModelsMaxPages; page++ {
		req, err := buildKiroListModelsRequest(ctx, account, token, region, nextToken)
		if err != nil {
			return nil, 0, err
		}
		resp, err := do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("list available models request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("read list available models response: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, resp.StatusCode, fmt.Errorf("list available models: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var parsed kiroListAvailableModelsResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("decode list available models response: %w", err)
		}
		for _, model := range parsed.Models {
			id := strings.TrimSpace(model.ModelID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		nextToken = strings.TrimSpace(parsed.NextToken)
		if nextToken == "" {
			return ids, http.StatusOK, nil
		}
	}
	return ids, http.StatusOK, nil
}

func buildKiroListModelsRequest(ctx context.Context, account *Account, token, region, nextToken string) (*http.Request, error) {
	query := url.Values{}
	query.Set("origin", kiroListModelsOrigin)
	query.Set("maxResults", kiroListModelsPageSize)
	if profileArn := strings.TrimSpace(kiroResolveRequestProfileArn(account)); profileArn != "" {
		query.Set("profileArn", profileArn)
	}
	if tokenValue := strings.TrimSpace(nextToken); tokenValue != "" {
		query.Set("nextToken", tokenValue)
	}
	endpoint := fmt.Sprintf("https://q.%s.amazonaws.com/ListAvailableModels?%s", region, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create list models request: %w", err)
	}
	accountKey := buildKiroAccountKey(account)
	machineID := buildKiroMachineID(account)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", kiropkg.BuildRuntimeUserAgent(accountKey, machineID))
	req.Header.Set("X-Amz-User-Agent", kiropkg.BuildRuntimeAmzUserAgent(accountKey, machineID))
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.NewString())
	applyKiroConditionalHeaders(req, account)
	return req, nil
}

func (s *AccountTestService) fetchKiroDirectUpstreamModels(ctx context.Context, account *Account) ([]string, error) {
	token, err := s.kiroSyncAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	ids, err := kiroListAvailableModelIDs(ctx, account, token)
	if err != nil {
		return nil, newUpstreamModelSyncUpstreamError("Failed to fetch Kiro available models", err)
	}
	names, _ := kiroPublicCatalog(ids)
	if len(names) == 0 {
		return nil, newUpstreamModelSyncUpstreamError("Upstream returned no supported models", nil)
	}
	return names, nil
}

func (s *AccountTestService) syncKiroDirectModelCatalog(ctx context.Context, account *Account) (*UpstreamModelCatalog, error) {
	token, err := s.kiroSyncAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	ids, err := kiroListAvailableModelIDs(ctx, account, token)
	if err != nil {
		return nil, newUpstreamModelSyncUpstreamError("Failed to fetch Kiro available models", err)
	}
	names, metadata := kiroPublicCatalog(ids)
	if len(names) == 0 {
		return nil, newUpstreamModelSyncUpstreamError("Upstream returned no supported models", nil)
	}
	return &UpstreamModelCatalog{Models: names, Metadata: metadata}, nil
}

func (s *AccountTestService) kiroSyncAccessToken(ctx context.Context, account *Account) (string, error) {
	if account != nil && account.Type == AccountTypeAPIKey {
		token := firstKiroCredential(account, "kiro_api_key", "kiroApiKey", "api_key")
		if token == "" {
			return "", newUpstreamModelSyncConfigError("No Kiro API key is available", nil)
		}
		return token, nil
	}
	if s == nil || s.kiroTokenProvider == nil {
		return "", newUpstreamModelSyncConfigError("Kiro token provider is not configured", nil)
	}
	token, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return "", newUpstreamModelSyncUpstreamError("Failed to get Kiro access token", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", newUpstreamModelSyncConfigError("No Kiro access token is available", nil)
	}
	return token, nil
}

// kiroSyncPublicModelNames 把上游 modelId 展开成对外名。Claude 紧跟一行 -thinking。
func kiroSyncPublicModelNames(ids []string) []string {
	names, _ := kiroPublicCatalog(ids)
	return names
}

// kiroPublicCatalog 展开对外名，并在 metadata 里记下原始 modelId。
// thinking 别名指向同一个上游 ID。界面用这个 ID 保存映射，不再从短横线名字猜点号位置。
func kiroPublicCatalog(ids []string) ([]string, map[string]UpstreamModelMetadata) {
	seen := make(map[string]struct{})
	names := make([]string, 0, len(ids))
	metadata := make(map[string]UpstreamModelMetadata)
	for _, id := range ids {
		for _, pair := range kiropkg.SyncModelAliases(id) {
			name := pair[0]
			if name == "" {
				continue
			}
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				names = append(names, name)
			}
			if _, ok := metadata[name]; ok {
				continue
			}
			metadata[name] = UpstreamModelMetadata{ID: pair[1]}
		}
	}
	return names, metadata
}

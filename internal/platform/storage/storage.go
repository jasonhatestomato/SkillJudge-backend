package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"skilljudge/backend/internal/config"

	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

type Credential struct {
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	SecurityToken   string `json:"securityToken"`
	Expiration      string `json:"expiration"`
}

type UploadedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

type CreateMultipartUploadInput struct {
	ObjectKey    string
	ObjectPrefix string
	Filename     string
	FileSize     int64
}

type MultipartUploadSession struct {
	UploadURL  string
	UploadID   string
	ObjectKey  string
	Credential Credential
}

type CompleteMultipartUploadInput struct {
	ObjectKey string
	UploadID  string
	Parts     []UploadedPart
}

type CompleteMultipartUploadResult struct {
	StorageURL  string
	StoragePath string
}

type PutObjectInput struct {
	ObjectKey   string
	ContentType string
	Body        []byte
}

type PutObjectResult struct {
	StorageURL  string
	StoragePath string
}

type Provider interface {
	CreateMultipartUpload(ctx context.Context, input CreateMultipartUploadInput) (*MultipartUploadSession, error)
	CompleteMultipartUpload(ctx context.Context, input CompleteMultipartUploadInput) (*CompleteMultipartUploadResult, error)
	PutObject(ctx context.Context, input PutObjectInput) (*PutObjectResult, error)
	DeleteObject(ctx context.Context, objectKey string) error
	GeneratePlayURL(ctx context.Context, objectKey string, expires time.Duration) (string, error)
}

func NewProvider(cfg config.StorageConfig) Provider {
	switch strings.ToLower(cfg.Provider) {
	case "obs", "huaweicloud_obs", "huawei_obs":
		if provider, err := newOBSProvider(cfg); err == nil {
			return provider
		}
		return newMockProvider(cfg)
	case "", "mock":
		return newMockProvider(cfg)
	default:
		return newMockProvider(cfg)
	}
}

type mockProvider struct {
	publicBaseURL string
	bucket        string
	region        string
}

func newMockProvider(cfg config.StorageConfig) Provider {
	baseURL := strings.TrimRight(cfg.PublicBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9000"
	}

	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "skilljudge-videos"
	}

	return &mockProvider{
		publicBaseURL: baseURL,
		bucket:        bucket,
		region:        cfg.Region,
	}
}

func (p *mockProvider) CreateMultipartUpload(_ context.Context, input CreateMultipartUploadInput) (*MultipartUploadSession, error) {
	uploadID := fmt.Sprintf("mock-upload-%d", time.Now().UnixNano())
	return &MultipartUploadSession{
		UploadURL: fmt.Sprintf("%s/%s/%s", p.publicBaseURL, p.bucket, input.ObjectKey),
		UploadID:  uploadID,
		ObjectKey: input.ObjectKey,
		Credential: Credential{
			AccessKeyID:     "mock-access-key-id",
			AccessKeySecret: "mock-access-key-secret",
			SecurityToken:   "mock-security-token",
			Expiration:      time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
		},
	}, nil
}

func (p *mockProvider) CompleteMultipartUpload(_ context.Context, input CompleteMultipartUploadInput) (*CompleteMultipartUploadResult, error) {
	return &CompleteMultipartUploadResult{
		StorageURL:  fmt.Sprintf("%s/%s/%s", p.publicBaseURL, p.bucket, input.ObjectKey),
		StoragePath: input.ObjectKey,
	}, nil
}

func (p *mockProvider) PutObject(_ context.Context, input PutObjectInput) (*PutObjectResult, error) {
	return &PutObjectResult{
		StorageURL:  fmt.Sprintf("%s/%s/%s", p.publicBaseURL, p.bucket, input.ObjectKey),
		StoragePath: input.ObjectKey,
	}, nil
}

func (p *mockProvider) DeleteObject(_ context.Context, _ string) error {
	return nil
}

func (p *mockProvider) GeneratePlayURL(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s/%s", p.publicBaseURL, p.bucket, objectKey), nil
}

type obsProvider struct {
	client          *obs.ObsClient
	httpClient      *http.Client
	publicBaseURL   string
	bucket          string
	iamEndpoint     string
	accessKeyID     string
	accessKeySecret string
	stsTTL          time.Duration
}

func newOBSProvider(cfg config.StorageConfig) (Provider, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("storage endpoint is required for obs provider")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("storage bucket is required for obs provider")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, fmt.Errorf("storage access key is required for obs provider")
	}

	client, err := obs.New(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("init obs client: %w", err)
	}

	publicBaseURL := strings.TrimRight(cfg.PublicBaseURL, "/")
	if publicBaseURL == "" {
		publicBaseURL = fmt.Sprintf("https://%s.%s", cfg.Bucket, cfg.Endpoint)
	}

	return &obsProvider{
		client:          client,
		httpClient:      config.NewHTTPClient(15 * time.Second),
		publicBaseURL:   publicBaseURL,
		bucket:          cfg.Bucket,
		iamEndpoint:     strings.TrimRight(cfg.IAMEndpoint, "/"),
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		stsTTL:          cfg.STSTTL,
	}, nil
}

func (p *obsProvider) CreateMultipartUpload(ctx context.Context, input CreateMultipartUploadInput) (*MultipartUploadSession, error) {
	req := &obs.InitiateMultipartUploadInput{}
	req.Bucket = p.bucket
	req.Key = input.ObjectKey
	resp, err := p.client.InitiateMultipartUpload(req)
	if err != nil {
		return nil, fmt.Errorf("initiate multipart upload: %w", err)
	}

	credential, err := p.issueTemporaryCredential(ctx, input.ObjectPrefix)
	if err != nil {
		return nil, fmt.Errorf("issue sts credential: %w", err)
	}

	return &MultipartUploadSession{
		// Frontend uploads parts directly to OBS; the backend only owns the
		// multipart session and restricted temporary credential issuance.
		UploadURL:  fmt.Sprintf("%s/%s", p.publicBaseURL, input.ObjectKey),
		UploadID:   resp.UploadId,
		ObjectKey:  input.ObjectKey,
		Credential: *credential,
	}, nil
}

func (p *obsProvider) CompleteMultipartUpload(_ context.Context, input CompleteMultipartUploadInput) (*CompleteMultipartUploadResult, error) {
	parts := make([]obs.Part, 0, len(input.Parts))
	for _, part := range input.Parts {
		parts = append(parts, obs.Part{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	req := &obs.CompleteMultipartUploadInput{}
	req.Bucket = p.bucket
	req.Key = input.ObjectKey
	req.UploadId = input.UploadID
	req.Parts = parts
	if _, err := p.client.CompleteMultipartUpload(req); err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	return &CompleteMultipartUploadResult{
		StorageURL:  fmt.Sprintf("%s/%s", p.publicBaseURL, input.ObjectKey),
		StoragePath: input.ObjectKey,
	}, nil
}

func (p *obsProvider) PutObject(_ context.Context, input PutObjectInput) (*PutObjectResult, error) {
	req := &obs.PutObjectInput{}
	req.Bucket = p.bucket
	req.Key = input.ObjectKey
	req.Body = bytes.NewReader(input.Body)
	if input.ContentType != "" {
		req.ContentType = input.ContentType
	}
	if _, err := p.client.PutObject(req); err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}

	return &PutObjectResult{
		StorageURL:  fmt.Sprintf("%s/%s", p.publicBaseURL, input.ObjectKey),
		StoragePath: input.ObjectKey,
	}, nil
}

func (p *obsProvider) DeleteObject(_ context.Context, objectKey string) error {
	req := &obs.DeleteObjectInput{}
	req.Bucket = p.bucket
	req.Key = objectKey
	if _, err := p.client.DeleteObject(req); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (p *obsProvider) GeneratePlayURL(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s", p.publicBaseURL, objectKey), nil
}

type stsRequest struct {
	Auth stsAuth `json:"auth"`
}

type stsAuth struct {
	Identity stsIdentity `json:"identity"`
}

type stsIdentity struct {
	Methods []string  `json:"methods"`
	Token   stsToken  `json:"token"`
	Policy  stsPolicy `json:"policy"`
}

type stsToken struct {
	DurationSeconds int `json:"duration_seconds"`
}

type stsPolicy struct {
	Version   string         `json:"Version"`
	Statement []stsStatement `json:"Statement"`
}

type stsStatement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

type stsResponse struct {
	Credential struct {
		Access        string `json:"access"`
		Secret        string `json:"secret"`
		SecurityToken string `json:"securitytoken"`
		ExpiresAt     string `json:"expires_at"`
	} `json:"credential"`
}

func (p *obsProvider) issueTemporaryCredential(ctx context.Context, objectPrefix string) (*Credential, error) {
	durationSeconds := int(p.stsTTL / time.Second)
	if durationSeconds <= 0 {
		durationSeconds = 1800
	}

	resourcePrefix := strings.TrimPrefix(objectPrefix, "/")
	resourcePrefix = strings.TrimSuffix(resourcePrefix, "/") + "/"
	// Restrict the temporary credential to the current upload prefix so the
	// client cannot write arbitrary objects in the bucket.
	resource := fmt.Sprintf("obs:*:*:object:%s/%s*", p.bucket, resourcePrefix)

	payload := stsRequest{
		Auth: stsAuth{
			Identity: stsIdentity{
				Methods: []string{"token"},
				Token: stsToken{
					DurationSeconds: durationSeconds,
				},
				Policy: stsPolicy{
					Version: "1.1",
					Statement: []stsStatement{
						{
							Effect: "Allow",
							Action: []string{
								"obs:object:PutObject",
							},
							Resource: []string{resource},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sts payload: %w", err)
	}

	requestURI := "/v3.0/OS-CREDENTIAL/securitytokens"
	canonicalURI := requestURI + "/"
	contentType := "application/json;charset=utf8"
	sdkDate := time.Now().UTC().Format("20060102T150405Z")

	endpointURL, err := url.Parse(p.iamEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse iam endpoint: %w", err)
	}
	host := endpointURL.Host

	bodyHash := sha256Hex(body)
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-sdk-date:%s\n", contentType, host, sdkDate)
	signedHeaders := "content-type;host;x-sdk-date"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		bodyHash,
	}, "\n")
	stringToSign := strings.Join([]string{
		"SDK-HMAC-SHA256",
		sdkDate,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hmacSHA256Hex([]byte(p.accessKeySecret), stringToSign)
	authorization := fmt.Sprintf(
		"SDK-HMAC-SHA256 Access=%s, SignedHeaders=%s, Signature=%s",
		p.accessKeyID,
		signedHeaders,
		signature,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.iamEndpoint+requestURI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sts request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Host", host)
	req.Header.Set("X-Sdk-Date", sdkDate)
	req.Header.Set("Authorization", authorization)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request sts credential: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sts response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sts response status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed stsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode sts response: %w", err)
	}
	if parsed.Credential.Access == "" || parsed.Credential.Secret == "" || parsed.Credential.SecurityToken == "" {
		return nil, fmt.Errorf("sts credential is incomplete")
	}

	return &Credential{
		AccessKeyID:     parsed.Credential.Access,
		AccessKeySecret: parsed.Credential.Secret,
		SecurityToken:   parsed.Credential.SecurityToken,
		Expiration:      parsed.Credential.ExpiresAt,
	}, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256Hex(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const defaultAWSDurationSeconds = int32(3600)

type STSServiceConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKey       string
	SecretKey       string
	RoleARN         string
	RoleSessionName string
	Duration        int32
	AllowPrefix     []string
	AllowActions    []string
}

type AWSSTSProvider struct {
	client          *sts.Client
	roleARN         string
	roleSessionName string
	duration        int32
	bucket          string
	prefixes        []string
	actions         []string
}

func NewAWSSTSProvider(cfg STSServiceConfig) (*AWSSTSProvider, error) {
	if strings.TrimSpace(cfg.Region) == "" || strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.RoleARN) == "" {
		return nil, nil
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if strings.TrimSpace(cfg.AccessKey) != "" || strings.TrimSpace(cfg.SecretKey) != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	if strings.TrimSpace(cfg.Endpoint) != "" {
		loadOptions = append(loadOptions, awsconfig.WithBaseEndpoint(strings.TrimSpace(cfg.Endpoint)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws sts config: %w", err)
	}

	roleSessionName := strings.TrimSpace(cfg.RoleSessionName)
	if roleSessionName == "" {
		roleSessionName = "grove-console"
	}
	actions := cfg.AllowActions
	if len(actions) == 0 {
		actions = []string{
			"s3:PutObject",
			"s3:GetObject",
			"s3:AbortMultipartUpload",
			"s3:ListBucketMultipartUploads",
			"s3:ListMultipartUploadParts",
		}
	}
	prefixes := cfg.AllowPrefix
	if len(prefixes) == 0 {
		prefixes = []string{"console/${user_id}"}
	}

	return &AWSSTSProvider{
		client:          sts.NewFromConfig(awsCfg),
		roleARN:         cfg.RoleARN,
		roleSessionName: roleSessionName,
		duration:        normalizeAWSDurationSeconds(cfg.Duration),
		bucket:          cfg.Bucket,
		prefixes:        prefixes,
		actions:         actions,
	}, nil
}

func (p *AWSSTSProvider) IssueToken(ctx context.Context, userID string) (*STSToken, error) {
	if p == nil {
		return nil, nil
	}

	resources := make([]string, 0, len(p.prefixes))
	for _, prefix := range p.prefixes {
		prefixPath := resolveScopedPrefix(prefix, userID)
		if prefixPath == "*" {
			resources = append(resources, fmt.Sprintf("arn:aws:s3:::%s/*", p.bucket))
			continue
		}
		resources = append(resources, fmt.Sprintf("arn:aws:s3:::%s/%s/*", p.bucket, strings.Trim(prefixPath, "/")))
	}

	policyBytes, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":   "Allow",
				"Action":   p.actions,
				"Resource": resources,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sts policy: %w", err)
	}

	output, err := p.client.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(p.roleARN),
		RoleSessionName: aws.String(p.roleSessionName + "-" + sanitizeSessionID(userID)),
		DurationSeconds: aws.Int32(p.duration),
		Policy:          aws.String(string(policyBytes)),
	})
	if err != nil {
		return nil, fmt.Errorf("assume role for sts: %w", err)
	}
	if output.Credentials == nil {
		return nil, fmt.Errorf("sts credentials are empty")
	}

	return &STSToken{
		AccessKeyID:     aws.ToString(output.Credentials.AccessKeyId),
		SecretAccessKey: aws.ToString(output.Credentials.SecretAccessKey),
		SessionToken:    aws.ToString(output.Credentials.SessionToken),
		Expiration:      output.Credentials.Expiration.Unix(),
		Prefixes:        p.prefixes,
		Actions:         p.actions,
	}, nil
}

func normalizeAWSDurationSeconds(duration int32) int32 {
	if duration <= 0 {
		return defaultAWSDurationSeconds
	}
	return duration
}

func resolveScopedPrefix(prefix, userID string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "*"
	}
	if trimmed == "*" {
		return "*"
	}
	user := strings.TrimSpace(userID)
	if user == "" {
		user = "anonymous"
	}
	trimmed = strings.ReplaceAll(trimmed, "${user_id}", user)
	trimmed = strings.ReplaceAll(trimmed, "{user_id}", user)
	return strings.Trim(trimmed, "/")
}

func sanitizeSessionID(userID string) string {
	id := strings.TrimSpace(userID)
	if id == "" {
		id = "anonymous"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", ".", "-")
	id = replacer.Replace(id)
	if len(id) > 48 {
		id = id[:48]
	}
	return id
}

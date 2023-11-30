package sqs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

type SQS struct {
	sqsService *sqs.Client
	QueueURL   string
}

func LoadSQSQueueURL() (string, string, error) {
	queueURL := os.Getenv("AWS_SQS_QUEUE_URL")
	if strings.EqualFold(queueURL, "") {
		return "", "", fmt.Errorf("AWS_SQS_QUEUE_URL must be set")
	}

	region := os.Getenv("AWS_REGION")
	if strings.EqualFold(region, "") {
		return "", "", fmt.Errorf("AWS_REGION must be set")
	}

	return queueURL, region, nil
}

func NewSQS(ctx context.Context, region, queueURL string) (*SQS, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("config.LoadDefaultConfig(ctx, config.WithRegion(%s): %w", region, err)
	}
	svc := sqs.NewFromConfig(cfg)

	return &SQS{
		sqsService: svc,
		QueueURL:   queueURL,
	}, nil
}

func (s *SQS) CreateTarget(ctx context.Context, target datastore.Target) error {
	return nil
}
func (s *SQS) GetTarget(ctx context.Context, id uuid.UUID) (*datastore.Target, error) {
	return &datastore.Target{
		ResourceType: datastore.ResourceTypeNano,
		Status:       datastore.TargetStatusActive,
	}, nil
}
func (s *SQS) GetTargetByScope(ctx context.Context, scope string) (*datastore.Target, error) {
	return &datastore.Target{
		ResourceType: datastore.ResourceTypeNano,
		Status:       datastore.TargetStatusActive,
	}, nil
}
func (s *SQS) ListTargets(ctx context.Context) ([]datastore.Target, error) {
	return nil, nil
}
func (s *SQS) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	return nil
}

// Deprecated: Use datastore.UpdateTargetStatus.
func (s *SQS) UpdateTargetStatus(ctx context.Context, targetID uuid.UUID, newStatus datastore.TargetStatus, description string) error {
	return nil
}
func (s *SQS) UpdateToken(ctx context.Context, targetID uuid.UUID, newToken string, newExpiredAt time.Time) error {
	return nil
}
func (s *SQS) UpdateTargetParam(ctx context.Context, targetID uuid.UUID, newResourceType datastore.ResourceType, newProviderURL sql.NullString) error {
	return nil
}

type datastoreJob struct {
	UUID           uuid.UUID `json:"uuid"`
	GHEDomain      string    `json:"ghe_domain"`
	Repository     string    `json:"repository"` // repo (:owner/:repo)
	CheckEventJSON string    `json:"check_event"`
	TargetID       uuid.UUID `json:"target_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (dj datastoreJob) marshal() (string, error) {
	v, err := json.Marshal(dj)
	if err != nil {
		return "", fmt.Errorf("json.Marshal(): %w", err)
	}
	return string(v), nil
}

func unmarshalDatastoreJob(in []byte) (*datastoreJob, error) {
	dj := datastoreJob{}
	if err := json.Unmarshal(in, &dj); err != nil {
		return nil, fmt.Errorf("json.Unmarshal(): %w", err)
	}
	return &dj, nil
}

func unmarshalDatastoreJobFromJob(in datastore.Job) *datastoreJob {
	dj := datastoreJob{
		UUID:           in.UUID,
		GHEDomain:      in.GHEDomain.String,
		Repository:     in.Repository,
		CheckEventJSON: in.CheckEventJSON,
		TargetID:       in.TargetID,
		CreatedAt:      in.CreatedAt,
		UpdatedAt:      in.UpdatedAt,
	}
	return &dj
}

func (s *SQS) EnqueueJob(ctx context.Context, job datastore.Job) error {
	dj := unmarshalDatastoreJobFromJob(job)
	body, err := dj.marshal()
	if err != nil {
		return fmt.Errorf("dj.marshal() (dj: %v): %w", dj, err)
	}
	in := &sqs.SendMessageInput{
		MessageBody:            aws.String(body),
		QueueUrl:               aws.String(s.QueueURL),
		MessageGroupId:         aws.String("myshoes"),
		MessageDeduplicationId: aws.String(job.UUID.String()),
	}

	if _, err := s.sqsService.SendMessage(ctx, in); err != nil {
		return fmt.Errorf("SendMessage(): %w", err)
	}

	return nil
}
func (s *SQS) ListJobs(ctx context.Context) ([]datastore.Job, error) {
	msg, err := s.sqsService.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.QueueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		return nil, fmt.Errorf("s.sqsService.ReceiveMessage(): %w", err)
	}

	if len(msg.Messages) == 0 {
		return nil, nil
	}

	var jobs []datastore.Job
	for _, m := range msg.Messages {
		j, err := ConvertJob([]byte(*m.Body))
		if err != nil {
			return nil, fmt.Errorf("ConvertJob(): %w", err)
		}

		jobs = append(jobs, *j)
	}

	if err != nil {
		return nil, fmt.Errorf("s.sqsService.ReceiveMessage(): %w", err)
	}
	return jobs, nil
}

func ConvertJob(body []byte) (*datastore.Job, error) {
	dj, err := unmarshalDatastoreJob(body)
	if err != nil {
		return nil, fmt.Errorf("unmarshalDatastoreJob(): %w", err)
	}

	gheDomain := sql.NullString{String: "", Valid: false}
	if !strings.EqualFold(dj.GHEDomain, "") {
		gheDomain = sql.NullString{String: dj.GHEDomain, Valid: true}
	}

	return &datastore.Job{
		UUID:           dj.UUID,
		GHEDomain:      gheDomain,
		Repository:     dj.Repository,
		CheckEventJSON: dj.CheckEventJSON,
		TargetID:       dj.TargetID,
		CreatedAt:      dj.CreatedAt,
		UpdatedAt:      dj.UpdatedAt,
	}, nil
}

func (s *SQS) DeleteJob(ctx context.Context, id uuid.UUID) error {
	msg, err := s.sqsService.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.QueueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		return fmt.Errorf("s.sqsService.ReceiveMessage(): %w", err)
	}

	for _, m := range msg.Messages {
		dj, err := unmarshalDatastoreJob([]byte(*m.Body))
		if err != nil {
			return fmt.Errorf("unmarshalDatastoreJob(): %w", err)
		}

		if dj.UUID == id {
			if _, err := s.sqsService.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(s.QueueURL),
				ReceiptHandle: m.ReceiptHandle,
			}); err != nil {
				return fmt.Errorf("s.sqsService.DeleteMessage(): %w", err)
			}
			return nil
		}
	}

	logger.Logf(false, "failed to delete job (id: %s): not found", id.String())
	return nil
}

func (s *SQS) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	return nil
}
func (s *SQS) ListRunners(ctx context.Context) ([]datastore.Runner, error) {
	return nil, nil
}
func (s *SQS) ListRunnersByTargetID(ctx context.Context, targetID uuid.UUID) ([]datastore.Runner, error) {
	return nil, nil
}
func (s *SQS) GetRunner(ctx context.Context, id uuid.UUID) (*datastore.Runner, error) {
	return nil, nil
}
func (s *SQS) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time, reason datastore.RunnerStatus) error {
	return nil
}

// Lock
func (s *SQS) GetLock(ctx context.Context) error {
	return nil
}
func (s *SQS) IsLocked(ctx context.Context) (string, error) {
	return "", nil
}

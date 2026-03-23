package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	accessdomain "github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	admincommands "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	admindomain "github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	adminports "github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	jobjobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	jobrunner "github.com/architectcgz/zhi-file-service-go/internal/services/job/infra/runner"
	jobports "github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	uploadcommands "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	uploaddomain "github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	uploadports "github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestRuntimeDeleteFlowConsumesOutboxAndFinalizesPhysicalDelete(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	state := newRuntimeFlowState()
	state.seedFile(runtimeFlowFile{
		FileID:          "file-1",
		TenantID:        "tenant-a",
		OwnerID:         "user-1",
		BlobID:          "blob-1",
		FileName:        "avatar.png",
		ContentType:     "image/png",
		SizeBytes:       182044,
		AccessLevel:     pkgstorage.AccessLevelPublic,
		Status:          accessdomain.FileStatusActive,
		ReferenceCount:  0,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-time.Hour),
		StorageProvider: pkgstorage.ProviderS3,
		BucketName:      "public-bucket",
		ObjectKey:       "tenant-a/uploads/file-1/avatar.png",
	})

	deleteFile := admincommands.NewDeleteFileHandler(
		runtimeAdminFileRepository{state: state},
		runtimeAuditRepository{state: state},
		state,
		runtimeNoopAdminTxManager{},
		&runtimeSequenceIDGenerator{ids: []string{"audit-1"}},
		clock.NewFixed(now),
	)

	result, err := deleteFile.Handle(context.Background(), admincommands.DeleteFileCommand{
		FileID:         "file-1",
		Reason:         "manual cleanup",
		IdempotencyKey: "delete-1",
		Auth:           newRuntimeAdminContext(t, admindomain.RoleGovernance, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.Status != "DELETED" {
		t.Fatalf("status = %q, want %q", result.Status, "DELETED")
	}
	if !state.hasPendingEvent(eventTypeFileDeleteRequested) {
		t.Fatalf("expected pending outbox event %q", eventTypeFileDeleteRequested)
	}

	scheduledJobs := buildScheduledJobs(runtimeJobConfig(24*time.Hour), scheduledJobDependencies{
		FileCleanup:    state,
		MultipartRepo:  state,
		OutboxConsumer: newOutboxConsumer(state, nil, clock.NewFixed(now.Add(48*time.Hour)), 10),
	}, clock.NewFixed(now.Add(48*time.Hour)))

	outboxRun := runScheduledJob(t, scheduledJobs, jobjobs.JobNameProcessOutboxEvents)
	if outboxRun.ItemsProcessed != 1 {
		t.Fatalf("process_outbox_events ItemsProcessed = %d, want 1", outboxRun.ItemsProcessed)
	}
	if outboxRun.RetryCount != 0 {
		t.Fatalf("process_outbox_events RetryCount = %d, want 0", outboxRun.RetryCount)
	}

	finalizeRun := runScheduledJob(t, scheduledJobs, jobjobs.JobNameFinalizeFileDelete)
	if finalizeRun.ItemsProcessed != 1 {
		t.Fatalf("finalize_file_delete ItemsProcessed = %d, want 1", finalizeRun.ItemsProcessed)
	}
	if !state.files["file-1"].PhysicallyDeleted {
		t.Fatalf("file %q was not physically deleted", "file-1")
	}
	if !state.isEventPublished(eventTypeFileDeleteRequested) {
		t.Fatalf("expected published outbox event %q", eventTypeFileDeleteRequested)
	}
}

func TestRuntimeFailedDirectUploadFlowConsumesOutboxAndCleansMultipart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC)
	state := newRuntimeFlowState()
	session := mustNewRuntimeSession(t, uploaddomain.CreateSessionParams{
		ID:               "upload-1",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "archive.zip",
		ContentType:      "application/zip",
		SizeBytes:        5242880,
		AccessLevel:      pkgstorage.AccessLevelPrivate,
		Mode:             uploaddomain.SessionModeDirect,
		TotalParts:       2,
		Object:           pkgstorage.ObjectRef{Provider: pkgstorage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-1/archive.zip"},
		ProviderUploadID: "provider-upload-1",
		Hash:             &uploaddomain.ContentHash{Algorithm: "SHA256", Value: runtimeValidHash()},
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
		ExpiresAt:        now.Add(30 * time.Minute),
	})
	state.seedUploadSession(session)
	state.uploadedParts[session.ID] = []pkgstorage.UploadedPart{
		{PartNumber: 1, ETag: "etag-1", SizeBytes: 2621440},
		{PartNumber: 2, ETag: "etag-2", SizeBytes: 2621440},
	}
	state.objectMetadata[session.Object.ObjectKey] = pkgstorage.ObjectMetadata{
		SizeBytes:   session.SizeBytes,
		ContentType: session.ContentType,
		ETag:        `"etag-final"`,
		Checksum:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	completeUpload := uploadcommands.NewCompleteUploadSessionHandler(
		runtimeUploadSessionRepository{state: state},
		nil,
		nil,
		nil,
		nil,
		nil,
		state,
		runtimeNoopUploadTxManager{},
		state,
		state,
		&runtimeSequenceIDGenerator{},
		clock.NewFixed(now),
	)

	_, err := completeUpload.Handle(context.Background(), uploadcommands.CompleteUploadSessionCommand{
		UploadSessionID: session.ID,
		UploadedParts: []pkgstorage.UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 2621440},
			{PartNumber: 2, ETag: "etag-2", SizeBytes: 2621440},
		},
		RequestID:      "req-upload-failed-1",
		IdempotencyKey: "complete-failed-1",
		Auth:           newRuntimeUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != uploaddomain.CodeUploadHashMismatch {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, uploaddomain.CodeUploadHashMismatch, err)
	}
	if state.sessions[session.ID].Status != uploaddomain.SessionStatusFailed {
		t.Fatalf("status = %s, want %s", state.sessions[session.ID].Status, uploaddomain.SessionStatusFailed)
	}
	if state.multipartCompleteCalls != 1 {
		t.Fatalf("multipart complete calls = %d, want 1", state.multipartCompleteCalls)
	}
	if !state.hasPendingEvent(eventTypeUploadSessionFailed) {
		t.Fatalf("expected pending outbox event %q", eventTypeUploadSessionFailed)
	}

	scheduledJobs := buildScheduledJobs(runtimeJobConfig(24*time.Hour), scheduledJobDependencies{
		FileCleanup:    state,
		MultipartRepo:  state,
		OutboxConsumer: newOutboxConsumer(state, nil, clock.NewFixed(now.Add(3*time.Hour)), 10),
	}, clock.NewFixed(now.Add(3*time.Hour)))

	outboxRun := runScheduledJob(t, scheduledJobs, jobjobs.JobNameProcessOutboxEvents)
	if outboxRun.ItemsProcessed != 1 {
		t.Fatalf("process_outbox_events ItemsProcessed = %d, want 1", outboxRun.ItemsProcessed)
	}
	if outboxRun.RetryCount != 0 {
		t.Fatalf("process_outbox_events RetryCount = %d, want 0", outboxRun.RetryCount)
	}

	cleanupRun := runScheduledJob(t, scheduledJobs, jobjobs.JobNameCleanupMultipart)
	if cleanupRun.ItemsProcessed != 1 {
		t.Fatalf("cleanup_multipart ItemsProcessed = %d, want 1", cleanupRun.ItemsProcessed)
	}
	if got := state.sessions[session.ID].ProviderUploadID; got != "" {
		t.Fatalf("ProviderUploadID = %q, want empty after cleanup", got)
	}
	if !state.isEventPublished(eventTypeUploadSessionFailed) {
		t.Fatalf("expected published outbox event %q", eventTypeUploadSessionFailed)
	}
}

type runtimeFlowState struct {
	files                  map[string]*runtimeFlowFile
	sessions               map[string]*uploaddomain.Session
	uploadedParts          map[string][]pkgstorage.UploadedPart
	objectMetadata         map[string]pkgstorage.ObjectMetadata
	outboxOrder            []string
	outboxEvents           map[string]*runtimeOutboxRecord
	nextOutboxEvent        int
	auditLogs              []adminports.AuditLogRecord
	multipartCompleteCalls int
	multipartCleanupCalls  int
}

type runtimeFlowFile struct {
	FileID            string
	TenantID          string
	OwnerID           string
	BlobID            string
	FileName          string
	ContentType       string
	SizeBytes         int64
	AccessLevel       pkgstorage.AccessLevel
	Status            accessdomain.FileStatus
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ReferenceCount    int
	StorageProvider   pkgstorage.Provider
	BucketName        string
	ObjectKey         string
	PhysicallyDeleted bool
}

type runtimeOutboxRecord struct {
	event       jobports.OutboxEvent
	createdAt   time.Time
	publishedAt *time.Time
	lastError   string
}

type runtimeAdminFileRepository struct {
	state *runtimeFlowState
}

type runtimeAuditRepository struct {
	state *runtimeFlowState
}

type runtimeUploadSessionRepository struct {
	state *runtimeFlowState
}

func newRuntimeFlowState() *runtimeFlowState {
	return &runtimeFlowState{
		files:          make(map[string]*runtimeFlowFile),
		sessions:       make(map[string]*uploaddomain.Session),
		uploadedParts:  make(map[string][]pkgstorage.UploadedPart),
		objectMetadata: make(map[string]pkgstorage.ObjectMetadata),
		outboxEvents:   make(map[string]*runtimeOutboxRecord),
	}
}

func runtimeJobConfig(fileDeleteRetention time.Duration) config.JobConfig {
	return config.JobConfig{
		DefaultBatchSize:            10,
		ProcessOutboxEventsInterval: 15 * time.Second,
		FinalizeFileDeleteInterval:  time.Minute,
		CleanupMultipartInterval:    10 * time.Minute,
		FileDeleteRetention:         fileDeleteRetention,
	}
}

func runScheduledJob(t *testing.T, scheduledJobs []jobrunner.ScheduledJob, jobName string) jobjobs.Result {
	t.Helper()

	for _, scheduled := range scheduledJobs {
		if scheduled.Job == nil || scheduled.Job.Name() != jobName {
			continue
		}
		if scheduled.Interval <= 0 {
			t.Fatalf("%s interval = %s, want > 0", jobName, scheduled.Interval)
		}
		result, err := jobjobs.Execute(context.Background(), scheduled.Job)
		if err != nil {
			t.Fatalf("%s Execute() error = %v", jobName, err)
		}
		return result
	}

	t.Fatalf("scheduled job %q not found", jobName)
	return jobjobs.Result{}
}

func (s *runtimeFlowState) seedFile(file runtimeFlowFile) {
	cloned := file
	s.files[file.FileID] = &cloned
}

func (s *runtimeFlowState) seedUploadSession(session *uploaddomain.Session) {
	s.sessions[session.ID] = session
}

func (s *runtimeFlowState) hasPendingEvent(eventType string) bool {
	for _, eventID := range s.outboxOrder {
		record := s.outboxEvents[eventID]
		if record == nil || record.event.EventType != eventType || record.publishedAt != nil {
			continue
		}
		return true
	}
	return false
}

func (s *runtimeFlowState) isEventPublished(eventType string) bool {
	for _, eventID := range s.outboxOrder {
		record := s.outboxEvents[eventID]
		if record == nil || record.event.EventType != eventType || record.publishedAt == nil {
			continue
		}
		return true
	}
	return false
}

func (r runtimeAdminFileRepository) GetByID(_ context.Context, fileID string) (*adminports.AdminFileView, error) {
	file := r.state.files[fileID]
	if file == nil {
		return nil, nil
	}
	return runtimeAdminFileView(file), nil
}

func (runtimeAdminFileRepository) List(context.Context, adminports.ListFilesQuery) ([]adminports.AdminFileView, string, error) {
	return nil, "", nil
}

func (r runtimeAdminFileRepository) MarkDeleted(_ context.Context, fileID string, deletedAt time.Time) (*adminports.DeleteFileRecord, error) {
	file := r.state.files[fileID]
	if file == nil {
		return nil, nil
	}
	if file.Status == accessdomain.FileStatusDeleted {
		return &adminports.DeleteFileRecord{
			File:                    *runtimeAdminFileView(file),
			PhysicalDeleteScheduled: true,
			AlreadyDeleted:          true,
		}, nil
	}

	file.Status = accessdomain.FileStatusDeleted
	file.DeletedAt = cloneTime(&deletedAt)
	file.UpdatedAt = deletedAt.UTC()

	return &adminports.DeleteFileRecord{
		File:                    *runtimeAdminFileView(file),
		PhysicalDeleteScheduled: true,
	}, nil
}

func (r runtimeAuditRepository) Append(_ context.Context, record adminports.AuditLogRecord) error {
	r.state.auditLogs = append(r.state.auditLogs, record)
	return nil
}

func (runtimeAuditRepository) List(context.Context, adminports.ListAuditLogsQuery) ([]adminports.AuditLogRecord, string, error) {
	return nil, "", nil
}

func (s *runtimeFlowState) Publish(_ context.Context, event adminports.OutboxEvent) error {
	s.nextOutboxEvent++
	eventID := runtimeEventID(s.nextOutboxEvent)
	s.outboxOrder = append(s.outboxOrder, eventID)
	s.outboxEvents[eventID] = &runtimeOutboxRecord{
		event: jobports.OutboxEvent{
			EventID:       eventID,
			ServiceName:   "admin-service",
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			EventType:     event.EventType,
			Payload:       cloneMap(event.Payload),
		},
		createdAt: event.OccurredAt.UTC(),
	}
	return nil
}

func (s *runtimeFlowState) ClaimPending(_ context.Context, query jobports.ClaimOutboxEventsQuery) ([]jobports.OutboxEvent, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	events := make([]jobports.OutboxEvent, 0, limit)
	for _, eventID := range s.outboxOrder {
		record := s.outboxEvents[eventID]
		if record == nil || record.publishedAt != nil {
			continue
		}

		dueAt := record.createdAt
		if record.event.NextAttemptAt != nil {
			dueAt = record.event.NextAttemptAt.UTC()
		}
		if dueAt.After(query.DueBefore.UTC()) {
			continue
		}
		if len(query.EventTypes) > 0 && !containsString(query.EventTypes, record.event.EventType) {
			continue
		}

		claimed := record.event
		nextAttemptAt := query.DueBefore.UTC().Add(time.Minute)
		claimed.NextAttemptAt = &nextAttemptAt
		record.event.NextAttemptAt = &nextAttemptAt
		events = append(events, claimed)
		if len(events) >= limit {
			break
		}
	}

	return events, nil
}

func (s *runtimeFlowState) MarkPublished(_ context.Context, eventID string, publishedAt time.Time) error {
	record := s.outboxEvents[eventID]
	if record == nil {
		return errors.New("outbox event not found")
	}
	record.publishedAt = cloneTime(&publishedAt)
	return nil
}

func (s *runtimeFlowState) MarkFailed(_ context.Context, eventID string, nextAttemptAt time.Time, lastError string) error {
	record := s.outboxEvents[eventID]
	if record == nil {
		return errors.New("outbox event not found")
	}
	record.event.RetryCount++
	record.event.NextAttemptAt = cloneTime(&nextAttemptAt)
	record.lastError = lastError
	return nil
}

func (s *runtimeFlowState) FinalizeDeletedFiles(_ context.Context, eligibleBefore time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	processed := 0
	for _, file := range s.files {
		if processed >= limit {
			break
		}
		if file.Status != accessdomain.FileStatusDeleted || file.DeletedAt == nil || file.PhysicallyDeleted {
			continue
		}
		if file.ReferenceCount != 0 || file.DeletedAt.After(eligibleBefore.UTC()) {
			continue
		}
		file.PhysicallyDeleted = true
		file.UpdatedAt = eligibleBefore.UTC()
		processed++
	}
	return processed, nil
}

func (s *runtimeFlowState) CleanupMultipartUploads(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	processed := 0
	for _, session := range s.sessions {
		if processed >= limit {
			break
		}
		if session.ProviderUploadID == "" || session.UpdatedAt.After(staleBefore.UTC()) {
			continue
		}
		switch session.Status {
		case uploaddomain.SessionStatusAborted, uploaddomain.SessionStatusExpired, uploaddomain.SessionStatusFailed:
			session.ProviderUploadID = ""
			session.UpdatedAt = staleBefore.UTC()
			s.multipartCleanupCalls++
			processed++
		}
	}
	return processed, nil
}

func (s *runtimeFlowState) Enqueue(_ context.Context, message uploadports.OutboxMessage) error {
	payload := make(map[string]any)
	if len(message.Payload) > 0 {
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return err
		}
	}

	s.nextOutboxEvent++
	eventID := runtimeEventID(s.nextOutboxEvent)
	s.outboxOrder = append(s.outboxOrder, eventID)
	s.outboxEvents[eventID] = &runtimeOutboxRecord{
		event: jobports.OutboxEvent{
			EventID:       eventID,
			ServiceName:   "upload-service",
			AggregateType: "upload_session",
			AggregateID:   message.AggregateID,
			EventType:     message.EventType,
			Payload:       payload,
		},
		createdAt: runtimeOccurredAt(payload),
	}
	return nil
}

func (r runtimeUploadSessionRepository) Create(_ context.Context, session *uploaddomain.Session) error {
	r.state.sessions[session.ID] = session
	return nil
}

func (r runtimeUploadSessionRepository) Save(_ context.Context, session *uploaddomain.Session) error {
	r.state.sessions[session.ID] = session
	return nil
}

func (r runtimeUploadSessionRepository) GetByID(_ context.Context, tenantID string, uploadSessionID string) (*uploaddomain.Session, error) {
	session := r.state.sessions[uploadSessionID]
	if session == nil || session.TenantID != tenantID {
		return nil, nil
	}
	return session, nil
}

func (runtimeUploadSessionRepository) FindReusable(context.Context, uploadports.ReusableSessionQuery) (*uploaddomain.Session, error) {
	return nil, nil
}

func (r runtimeUploadSessionRepository) AcquireCompletion(_ context.Context, request uploadports.CompletionAcquireRequest) (*uploadports.CompletionAcquireResult, error) {
	session := r.state.sessions[request.UploadSessionID]
	if session == nil || session.TenantID != request.TenantID {
		return nil, nil
	}
	return &uploadports.CompletionAcquireResult{
		Session:   session,
		Ownership: uploaddomain.CompletionOwnershipAcquired,
	}, nil
}

func (r runtimeUploadSessionRepository) ConfirmCompletionOwner(_ context.Context, tenantID string, uploadSessionID string, completionToken string) (*uploaddomain.Session, error) {
	session := r.state.sessions[uploadSessionID]
	if session == nil || session.TenantID != tenantID {
		return nil, nil
	}
	if session.CompletionToken != completionToken {
		return nil, nil
	}
	return session, nil
}

func (s *runtimeFlowState) CreateMultipartUpload(context.Context, pkgstorage.ObjectRef, string) (string, error) {
	return "", errors.New("unexpected call")
}

func (s *runtimeFlowState) UploadPart(context.Context, pkgstorage.ObjectRef, string, int, io.Reader, int64) (string, error) {
	return "", errors.New("unexpected call")
}

func (s *runtimeFlowState) ListUploadedParts(_ context.Context, ref pkgstorage.ObjectRef, _ string) ([]pkgstorage.UploadedPart, error) {
	for _, session := range s.sessions {
		if session.Object.ObjectKey == ref.ObjectKey {
			return append([]pkgstorage.UploadedPart(nil), s.uploadedParts[session.ID]...), nil
		}
	}
	return nil, nil
}

func (s *runtimeFlowState) CompleteMultipartUpload(context.Context, pkgstorage.ObjectRef, string, []pkgstorage.UploadedPart) error {
	s.multipartCompleteCalls++
	return nil
}

func (s *runtimeFlowState) AbortMultipartUpload(context.Context, pkgstorage.ObjectRef, string) error {
	return nil
}

func (s *runtimeFlowState) HeadObject(_ context.Context, ref pkgstorage.ObjectRef) (pkgstorage.ObjectMetadata, error) {
	metadata, ok := s.objectMetadata[ref.ObjectKey]
	if !ok {
		return pkgstorage.ObjectMetadata{}, errors.New("object metadata not found")
	}
	return metadata, nil
}

func runtimeAdminFileView(file *runtimeFlowFile) *adminports.AdminFileView {
	if file == nil {
		return nil
	}
	return &adminports.AdminFileView{
		FileID:      file.FileID,
		TenantID:    file.TenantID,
		OwnerID:     file.OwnerID,
		BlobID:      file.BlobID,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		AccessLevel: file.AccessLevel,
		Status:      string(file.Status),
		DeletedAt:   cloneTime(file.DeletedAt),
		CreatedAt:   file.CreatedAt,
		UpdatedAt:   file.UpdatedAt,
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func runtimeEventID(seq int) string {
	return fmt.Sprintf("evt-%d", seq)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

type runtimeSequenceIDGenerator struct {
	ids   []string
	calls int
}

func (g *runtimeSequenceIDGenerator) New() (string, error) {
	if g.calls >= len(g.ids) {
		return "", errors.New("no more ids")
	}
	id := g.ids[g.calls]
	g.calls++
	return id, nil
}

type runtimeNoopAdminTxManager struct{}

func (runtimeNoopAdminTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type runtimeNoopUploadTxManager struct{}

func (runtimeNoopUploadTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func newRuntimeAdminContext(t *testing.T, role admindomain.Role, scope string) admindomain.AdminContext {
	t.Helper()

	auth, err := admindomain.NewAdminContext(admindomain.AdminContextInput{
		RequestID:    "req-admin-runtime-1",
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: []string{scope},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}
	return auth
}

func newRuntimeUploadAuth() uploaddomain.AuthContext {
	return uploaddomain.AuthContext{
		RequestID: "req-upload-runtime-1",
		SubjectID: "user-1",
		TenantID:  "tenant-a",
		Scopes:    []string{uploaddomain.ScopeFileWrite},
	}
}

func runtimeValidHash() string {
	return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}

func mustNewRuntimeSession(t *testing.T, params uploaddomain.CreateSessionParams) *uploaddomain.Session {
	t.Helper()

	session, err := uploaddomain.NewSession(params)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func runtimeOccurredAt(payload map[string]any) time.Time {
	raw, ok := payload["occurredAt"]
	if !ok {
		return time.Now().UTC()
	}

	value, ok := raw.(string)
	if !ok {
		return time.Now().UTC()
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed.UTC()
}

var (
	_ adminports.AdminFileRepository = runtimeAdminFileRepository{}
	_ adminports.AuditLogRepository   = runtimeAuditRepository{}
	_ adminports.OutboxPublisher      = (*runtimeFlowState)(nil)
	_ jobports.OutboxReader           = (*runtimeFlowState)(nil)
	_ jobports.FileCleanupRepository  = (*runtimeFlowState)(nil)
	_ jobports.MultipartRepository    = (*runtimeFlowState)(nil)
	_ uploadports.SessionRepository   = runtimeUploadSessionRepository{}
	_ uploadports.OutboxPublisher     = (*runtimeFlowState)(nil)
	_ uploadports.MultipartManager    = (*runtimeFlowState)(nil)
	_ uploadports.ObjectReader        = (*runtimeFlowState)(nil)
)

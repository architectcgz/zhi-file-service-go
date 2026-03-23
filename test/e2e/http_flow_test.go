package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	admincommands "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	adminview "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	admindomain "github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	adminhttp "github.com/architectcgz/zhi-file-service-go/internal/services/admin/transport/http"
	accessqueries "github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	accessdomain "github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	accesshttp "github.com/architectcgz/zhi-file-service-go/internal/services/access/transport/http"
	uploadcommands "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	uploadview "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	uploaddomain "github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	uploadhttp "github.com/architectcgz/zhi-file-service-go/internal/services/upload/transport/http"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestUploadThenAccessFileE2E(t *testing.T) {
	t.Parallel()

	state := newDeliveryState()
	uploadServer := newUploadServer(state)
	defer uploadServer.Close()
	accessServer := newAccessServer(state)
	defer accessServer.Close()

	client := uploadServer.Client()
	fileID := provisionFile(t, client, uploadServer.URL)

	req, err := http.NewRequest(http.MethodGet, accessServer.URL+"/api/v1/files/"+fileID, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-e2e-access-1")

	resp, err := accessServer.Client().Do(req)
	if err != nil {
		t.Fatalf("access get file request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	payload := decodeResponse(t, resp)
	data := payload["data"].(map[string]any)
	if data["fileId"] != fileID {
		t.Fatalf("fileId = %v, want %q", data["fileId"], fileID)
	}
	if data["downloadUrl"] != "https://cdn.example.com/public/tenant-a/avatar.png" {
		t.Fatalf("downloadUrl = %v, want public url", data["downloadUrl"])
	}

	redirectClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	redirectReq, err := http.NewRequest(http.MethodGet, accessServer.URL+"/api/v1/files/"+fileID+"/download?disposition=inline", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	redirectReq.Header.Set("Authorization", "Bearer token")
	redirectReq.Header.Set("X-Request-Id", "req-e2e-download-1")

	redirectResp, err := redirectClient.Do(redirectReq)
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer redirectResp.Body.Close()

	if redirectResp.StatusCode != http.StatusFound {
		t.Fatalf("download status = %d, want %d", redirectResp.StatusCode, http.StatusFound)
	}
	if got := redirectResp.Header.Get("Location"); got != "https://cdn.example.com/public/tenant-a/avatar.png" {
		t.Fatalf("Location = %q, want public url", got)
	}
}

func TestAdminDeleteBlocksAccessE2E(t *testing.T) {
	t.Parallel()

	state := newDeliveryState()
	uploadServer := newUploadServer(state)
	defer uploadServer.Close()
	accessServer := newAccessServer(state)
	defer accessServer.Close()
	adminServer := newAdminServer(state)
	defer adminServer.Close()

	fileID := provisionFile(t, uploadServer.Client(), uploadServer.URL)

	deleteReq, err := http.NewRequest(http.MethodDelete, adminServer.URL+"/api/admin/v1/files/"+fileID, strings.NewReader(`{"reason":"manual cleanup"}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer token")
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("Idempotency-Key", "delete-e2e-1")
	deleteReq.Header.Set("X-Request-Id", "req-e2e-delete-1")

	deleteResp, err := adminServer.Client().Do(deleteReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d, want %d", deleteResp.StatusCode, http.StatusOK)
	}
	deletePayload := decodeResponse(t, deleteResp)
	deleteData := deletePayload["data"].(map[string]any)
	if deleteData["status"] != "DELETED" {
		t.Fatalf("status = %v, want %q", deleteData["status"], "DELETED")
	}

	getReq, err := http.NewRequest(http.MethodGet, accessServer.URL+"/api/v1/files/"+fileID, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	getReq.Header.Set("Authorization", "Bearer token")
	getReq.Header.Set("X-Request-Id", "req-e2e-access-2")

	getResp, err := accessServer.Client().Do(getReq)
	if err != nil {
		t.Fatalf("access request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", getResp.StatusCode, http.StatusNotFound)
	}
	payload := decodeResponse(t, getResp)
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(accessdomain.CodeFileNotFound) {
		t.Fatalf("error.code = %v, want %q", errPayload["code"], accessdomain.CodeFileNotFound)
	}
}

type deliveryState struct {
	mu       sync.Mutex
	now      time.Time
	nextFile int
	nextSess int
	files    map[string]*deliveryFile
	sessions map[string]*deliverySession
}

type deliveryFile struct {
	FileID      string
	TenantID    string
	FileName    string
	ContentType string
	SizeBytes   int64
	AccessLevel pkgstorage.AccessLevel
	Status      accessdomain.FileStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type deliverySession struct {
	UploadSessionID string
	TenantID        string
	OwnerID         string
	FileName        string
	ContentType     string
	SizeBytes       int64
	AccessLevel     pkgstorage.AccessLevel
	UploadMode      uploaddomain.SessionMode
	Status          uploaddomain.SessionStatus
	TotalParts      int
	UploadedParts   int
	FileID      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

func newDeliveryState() *deliveryState {
	return &deliveryState{
		now:      time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
		files:    make(map[string]*deliveryFile),
		sessions: make(map[string]*deliverySession),
	}
}

func (s *deliveryState) createUploadSession(_ context.Context, command uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSess++
	s.now = s.now.Add(time.Second)
	sessionID := fmt.Sprintf("upload-%d", s.nextSess)
	totalParts := command.TotalParts
	if totalParts == 0 {
		totalParts = 1
	}

	session := &deliverySession{
		UploadSessionID: sessionID,
		TenantID:        command.Auth.TenantID,
		OwnerID:         command.Auth.SubjectID,
		FileName:        command.FileName,
		ContentType:     command.ContentType,
		SizeBytes:       command.SizeBytes,
		AccessLevel:     command.AccessLevel,
		UploadMode:      command.UploadMode,
		Status:          uploaddomain.SessionStatusInitiated,
		TotalParts:      totalParts,
		CreatedAt:       s.now,
		UpdatedAt:       s.now,
	}
	s.sessions[sessionID] = session

	return session.toView(), nil
}

func (s *deliveryState) completeUploadSession(_ context.Context, command uploadcommands.CompleteUploadSessionCommand) (uploadview.CompletedUploadSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[command.UploadSessionID]
	if session == nil {
		return uploadview.CompletedUploadSession{}, uploaddomain.ErrUploadSessionNotFound(command.UploadSessionID)
	}

	s.now = s.now.Add(time.Second)
	if session.FileID == "" {
		s.nextFile++
		session.FileID = fmt.Sprintf("file-%d", s.nextFile)
		s.files[session.FileID] = &deliveryFile{
			FileID:      session.FileID,
			TenantID:    session.TenantID,
			FileName:    session.FileName,
			ContentType: session.ContentType,
			SizeBytes:   session.SizeBytes,
			AccessLevel: session.AccessLevel,
			Status:      accessdomain.FileStatusActive,
			CreatedAt:   s.now,
			UpdatedAt:   s.now,
		}
	}

	session.Status = uploaddomain.SessionStatusCompleted
	session.UploadedParts = max(len(command.UploadedParts), session.TotalParts)
	session.CompletedAt = timePtr(s.now)
	session.UpdatedAt = s.now

	return uploadview.CompletedUploadSession{
		FileID:        session.FileID,
		UploadSession: session.toView(),
	}, nil
}

func (s *deliveryState) getFile(_ context.Context, query accessqueries.GetFileQuery) (accessqueries.GetFileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.files[query.FileID]
	if file == nil || file.Status != accessdomain.FileStatusActive {
		return accessqueries.GetFileResult{}, accessdomain.ErrFileNotFound(query.FileID)
	}
	if query.Auth.TenantID != file.TenantID {
		return accessqueries.GetFileResult{}, accessdomain.ErrTenantScopeDenied(query.FileID)
	}

	result := accessqueries.GetFileResult{File: file.toView()}
	if file.AccessLevel == pkgstorage.AccessLevelPublic {
		result.DownloadURL = "https://cdn.example.com/public/tenant-a/avatar.png"
	}
	return result, nil
}

func (s *deliveryState) resolveDownload(_ context.Context, query accessqueries.ResolveDownloadQuery) (accessqueries.ResolveDownloadResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.files[query.FileID]
	if file == nil || file.Status != accessdomain.FileStatusActive {
		return accessqueries.ResolveDownloadResult{}, accessdomain.ErrFileNotFound(query.FileID)
	}
	if query.Auth.TenantID != file.TenantID {
		return accessqueries.ResolveDownloadResult{}, accessdomain.ErrTenantScopeDenied(query.FileID)
	}

	url := "https://storage.example.com/private/tenant-a/avatar.png?sig=1"
	if file.AccessLevel == pkgstorage.AccessLevelPublic {
		url = "https://cdn.example.com/public/tenant-a/avatar.png"
	}

	return accessqueries.ResolveDownloadResult{
		File: file.toView(),
		URL:  url,
	}, nil
}

func (s *deliveryState) deleteFile(_ context.Context, command admincommands.DeleteFileCommand) (adminview.DeleteFileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.files[command.FileID]
	if file == nil {
		return adminview.DeleteFileResult{}, admindomain.ErrFileNotFound(command.FileID)
	}

	s.now = s.now.Add(time.Second)
	file.Status = accessdomain.FileStatusDeleted
	file.UpdatedAt = s.now
	file.DeletedAt = timePtr(s.now)

	return adminview.DeleteFileResult{
		FileID:                  file.FileID,
		Status:                  "DELETED",
		DeletedAt:               file.DeletedAt,
		PhysicalDeleteScheduled: true,
	}, nil
}

func (s *deliverySession) toView() uploadview.UploadSession {
	return uploadview.UploadSession{
		UploadSessionID: s.UploadSessionID,
		TenantID:        s.TenantID,
		UploadMode:      s.UploadMode,
		Status:          s.Status,
		FileName:        s.FileName,
		ContentType:     s.ContentType,
		SizeBytes:       s.SizeBytes,
		AccessLevel:     s.AccessLevel,
		TotalParts:      s.TotalParts,
		UploadedParts:   s.UploadedParts,
		FileID:          s.FileID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		CompletedAt:     s.CompletedAt,
	}
}

func (f *deliveryFile) toView() accessdomain.FileView {
	return accessdomain.FileView{
		FileID:          f.FileID,
		TenantID:        f.TenantID,
		FileName:        f.FileName,
		ContentType:     f.ContentType,
		SizeBytes:       f.SizeBytes,
		AccessLevel:     f.AccessLevel,
		Status:          f.Status,
		StorageProvider: pkgstorage.ProviderS3,
		BucketName:      "tenant-a-bucket",
		ObjectKey:       "tenant-a/" + f.FileName,
		CreatedAt:       f.CreatedAt,
		UpdatedAt:       f.UpdatedAt,
	}
}

func newUploadServer(state *deliveryState) *httptest.Server {
	return httptest.NewServer(uploadhttp.NewHandler(uploadhttp.Options{
		Auth: func(*http.Request) (uploaddomain.AuthContext, error) {
			return uploaddomain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{uploaddomain.ScopeFileWrite},
			}, nil
		},
		CreateUploadSession:   uploadCreateFunc(state.createUploadSession),
		CompleteUploadSession: uploadCompleteFunc(state.completeUploadSession),
	}))
}

func newAccessServer(state *deliveryState) *httptest.Server {
	return httptest.NewServer(accesshttp.NewHandler(accesshttp.Options{
		Auth: func(*http.Request) (accessdomain.AuthContext, error) {
			return accessdomain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{accessdomain.ScopeFileRead},
			}, nil
		},
		GetFile:         accessGetFunc(state.getFile),
		ResolveDownload: accessResolveFunc(state.resolveDownload),
	}))
}

func newAdminServer(state *deliveryState) *httptest.Server {
	return httptest.NewServer(adminhttp.NewHandler(adminhttp.Options{
		Auth: func(*http.Request) (admindomain.AdminContext, error) {
			return mustNewAdminContext(admindomain.RoleGovernance, "tenant-a"), nil
		},
		DeleteFile: adminDeleteFunc(state.deleteFile),
	}))
}

func provisionFile(t *testing.T, client *http.Client, uploadBaseURL string) string {
	t.Helper()

	createReq, err := http.NewRequest(http.MethodPost, uploadBaseURL+"/api/v1/upload-sessions", strings.NewReader(`{
		"fileName":"avatar.png",
		"contentType":"image/png",
		"sizeBytes":182044,
		"accessLevel":"PUBLIC",
		"uploadMode":"PRESIGNED_SINGLE",
		"contentHash":{"algorithm":"SHA256","value":"4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"}
	}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	createReq.Header.Set("Authorization", "Bearer token")
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Request-Id", "req-e2e-upload-1")
	createReq.Header.Set("Idempotency-Key", "upload-e2e-1")

	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("create upload request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResp.StatusCode, http.StatusCreated)
	}
	createPayload := decodeResponse(t, createResp)
	sessionID := createPayload["data"].(map[string]any)["uploadSessionId"].(string)

	completeReq, err := http.NewRequest(http.MethodPost, uploadBaseURL+"/api/v1/upload-sessions/"+sessionID+"/complete", strings.NewReader(`{
		"contentHash":{"algorithm":"SHA256","value":"4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"},
		"uploadedParts":[]
	}`))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	completeReq.Header.Set("Authorization", "Bearer token")
	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("X-Request-Id", "req-e2e-upload-2")
	completeReq.Header.Set("Idempotency-Key", "upload-e2e-2")

	completeResp, err := client.Do(completeReq)
	if err != nil {
		t.Fatalf("complete upload request failed: %v", err)
	}
	defer completeResp.Body.Close()

	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("complete status = %d, want %d", completeResp.StatusCode, http.StatusOK)
	}
	completePayload := decodeResponse(t, completeResp)
	return completePayload["data"].(map[string]any)["fileId"].(string)
}

func decodeResponse(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	return payload
}

func mustNewAdminContext(role admindomain.Role, scopes ...string) admindomain.AdminContext {
	auth, err := admindomain.NewAdminContext(admindomain.AdminContextInput{
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: scopes,
	})
	if err != nil {
		panic(fmt.Sprintf("NewAdminContext() error = %v", err))
	}
	return auth
}

func timePtr(value time.Time) *time.Time {
	cloned := value.UTC()
	return &cloned
}

type uploadCreateFunc func(context.Context, uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error)

func (fn uploadCreateFunc) Handle(ctx context.Context, command uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error) {
	return fn(ctx, command)
}

type uploadCompleteFunc func(context.Context, uploadcommands.CompleteUploadSessionCommand) (uploadview.CompletedUploadSession, error)

func (fn uploadCompleteFunc) Handle(ctx context.Context, command uploadcommands.CompleteUploadSessionCommand) (uploadview.CompletedUploadSession, error) {
	return fn(ctx, command)
}

type accessGetFunc func(context.Context, accessqueries.GetFileQuery) (accessqueries.GetFileResult, error)

func (fn accessGetFunc) Handle(ctx context.Context, query accessqueries.GetFileQuery) (accessqueries.GetFileResult, error) {
	return fn(ctx, query)
}

type accessResolveFunc func(context.Context, accessqueries.ResolveDownloadQuery) (accessqueries.ResolveDownloadResult, error)

func (fn accessResolveFunc) Handle(ctx context.Context, query accessqueries.ResolveDownloadQuery) (accessqueries.ResolveDownloadResult, error) {
	return fn(ctx, query)
}

type adminDeleteFunc func(context.Context, admincommands.DeleteFileCommand) (adminview.DeleteFileResult, error)

func (fn adminDeleteFunc) Handle(ctx context.Context, command admincommands.DeleteFileCommand) (adminview.DeleteFileResult, error) {
	return fn(ctx, command)
}

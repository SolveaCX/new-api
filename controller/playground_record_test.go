package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const (
	playgroundRecordID       = "550e8400-e29b-41d4-a716-446655440000"
	playgroundConversationID = "550e8400-e29b-41d4-a716-446655440001"
)

func setupPlaygroundControllerDB(t *testing.T, userIDs ...int) {
	t.Helper()

	previous := model.DB
	db, err := gorm.Open(
		sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Asset{}, &model.AssetBinding{}, &model.PlaygroundRecord{}, &model.PlaygroundRecordAsset{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
	})

	for _, userID := range userIDs {
		require.NoError(t, db.Create(&model.User{
			Id:       userID,
			Username: fmt.Sprintf("playground-controller-%d", userID),
			AffCode:  fmt.Sprintf("playground-controller-aff-%d", userID),
		}).Error)
	}
}

func playgroundRecordTestContext(t *testing.T, userID int, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/playground/records", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	return c, recorder
}

func playgroundExportTestContext(t *testing.T, adminID int, query string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	path := "/api/playground/records/export"
	if query != "" {
		path += "?" + query
	}
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Set("id", adminID)
	return c, recorder
}

func validPlaygroundRecordBody() string {
	return `{
		"record_id":"550e8400-e29b-41d4-a716-446655440000",
		"conversation_id":"550e8400-e29b-41d4-a716-446655440001",
		"user_message":{"key":"u","from":"user","versions":[{"id":"uv","content":"hello"}]},
		"request_messages":[{"role":"user","content":"hello"}],
		"assistant_message":{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"},
		"reasoning_content":"thinking",
		"input_text":"hello",
		"output_text":"world",
		"model_name":"gpt-test",
		"group_name":"plg",
		"parameters":{"temperature":0.7},
		"status":"complete",
		"error_code":"",
		"error_message":"",
		"relay_request_id":"request-1",
		"prompt_tokens":2,
		"completion_tokens":3,
		"total_tokens":5,
		"latency_ms":120,
		"messages_snapshot":[{"key":"u","from":"user","versions":[{"id":"uv","content":"hello"}]},{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"}],
		"client_completed_at":1000
	}`
}

func TestSavePlaygroundRecordPersistsAuthenticatedUser(t *testing.T) {
	setupPlaygroundControllerDB(t, 201, 202)
	body := strings.Replace(validPlaygroundRecordBody(), "{", `{"user_id":202,`, 1)
	c, recorder := playgroundRecordTestContext(t, 201, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":null}`, recorder.Body.String())
	var stored model.PlaygroundRecord
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).First(&stored).Error)
	require.Equal(t, 201, stored.UserID)
	require.Equal(t, "world", string(stored.OutputText))
	require.Equal(t, 5, stored.TotalTokens)
}

func TestSavePlaygroundRecordRejectsBase64Media(t *testing.T) {
	setupPlaygroundControllerDB(t, 203)
	body := strings.Replace(
		validPlaygroundRecordBody(),
		`"content":"hello"`,
		`"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]`,
		1,
	)
	c, recorder := playgroundRecordTestContext(t, 203, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.PlaygroundRecord{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSavePlaygroundRecordAcceptsURLMedia(t *testing.T) {
	setupPlaygroundControllerDB(t, 204)
	body := strings.Replace(
		validPlaygroundRecordBody(),
		`"content":"hello"`,
		`"content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]`,
		1,
	)
	c, recorder := playgroundRecordTestContext(t, 204, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestSavePlaygroundRecordPersistsDurableAttachmentReferenceWithoutSignedURL(t *testing.T) {
	setupPlaygroundControllerDB(t, 214)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.Asset{
		PublicId: "ast_controller_attachment", UserId: 214, AssetType: model.AssetTypeImage,
		Status: model.AssetStatusActive, SourceStatus: model.AssetSourceStatusAvailable,
		StorageBackend: "gcs", StorageBucket: "bucket", ObjectKey: "object.png", ObjectGeneration: 1,
		ContentType: "image/png", SizeBytes: 1, SourceExpiresAt: now + 3600, CreatedAt: now, UpdatedAt: now,
	}).Error)
	body := `{
		"record_id":"550e8400-e29b-41d4-a716-446655440000",
		"conversation_id":"550e8400-e29b-41d4-a716-446655440001",
		"user_message":{"key":"u","from":"user","versions":[{"id":"uv","content":"hello","attachments":[{"kind":"image","filename":"frame.png","mediaType":"image/png","assetId":"ast_controller_attachment","url":"https://signed.example/frame.png"}]}]},
		"request_messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"https://signed.example/frame.png"}}]}],
		"assistant_message":{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"},
		"reasoning_content":"","input_text":"hello","output_text":"world","model_name":"gpt-test","group_name":"plg",
		"parameters":{},"status":"complete","error_code":"","error_message":"","relay_request_id":"","prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"latency_ms":1,
		"messages_snapshot":[{"key":"u","from":"user","versions":[{"id":"uv","content":"hello","attachments":[{"kind":"image","filename":"frame.png","mediaType":"image/png","assetId":"ast_controller_attachment","url":"https://signed.example/frame.png"}]}]},{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"}],
		"client_completed_at":1000
	}`
	c, recorder := playgroundRecordTestContext(t, 214, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var stored model.PlaygroundRecord
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).First(&stored).Error)
	require.NotContains(t, string(stored.UserMessage), "https://signed.example/frame.png")
	require.Contains(t, string(stored.RequestMessages), "asset://ast_controller_attachment")
	require.NotContains(t, string(stored.RequestMessages), "https://signed.example/frame.png")
	var references []model.PlaygroundRecordAsset
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).Find(&references).Error)
	require.Len(t, references, 1)
	require.Equal(t, "ast_controller_attachment", references[0].AssetID)
}

func TestSavePlaygroundRecordPersistsGeneratedAudioAssetReferenceWithoutSignedURL(t *testing.T) {
	setupPlaygroundControllerDB(t, 216)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.Asset{
		PublicId: "ast_controller_generated_audio", UserId: 216, AssetType: model.AssetTypeAudio,
		Status: model.AssetStatusActive, SourceStatus: model.AssetSourceStatusAvailable,
		StorageBackend: "gcs", StorageBucket: "bucket", ObjectKey: "generated.wav", ObjectGeneration: 1,
		ContentType: "audio/wav", SizeBytes: 4, SourceExpiresAt: now + 3600, CreatedAt: now, UpdatedAt: now,
	}).Error)
	body := `{
		"record_id":"550e8400-e29b-41d4-a716-446655440000",
		"conversation_id":"550e8400-e29b-41d4-a716-446655440001",
		"user_message":{"key":"u","from":"user","versions":[{"id":"uv","content":"say hello"}]},
		"request_messages":[{"role":"user","content":"say hello"}],
		"assistant_message":{"key":"a","from":"assistant","versions":[{"id":"av","content":"Audio","generatedMedia":[{"type":"audio","assetId":"ast_controller_generated_audio","mimeType":"audio/wav","url":"https://signed.example/generated.wav"}]}],"status":"complete"},
		"reasoning_content":"","input_text":"say hello","output_text":"Audio","model_name":"tts-1","group_name":"plg",
		"parameters":{},"status":"complete","error_code":"","error_message":"","relay_request_id":"","prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"latency_ms":1,
		"messages_snapshot":[{"key":"u","from":"user","versions":[{"id":"uv","content":"say hello"}]},{"key":"a","from":"assistant","versions":[{"id":"av","content":"Audio","generatedMedia":[{"type":"audio","assetId":"ast_controller_generated_audio","mimeType":"audio/wav","url":"https://signed.example/generated.wav"}]}],"status":"complete"}],
		"client_completed_at":1000
	}`
	c, recorder := playgroundRecordTestContext(t, 216, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var stored model.PlaygroundRecord
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).First(&stored).Error)
	require.NotContains(t, string(stored.AssistantMessage), "https://signed.example/generated.wav")
	require.NotContains(t, string(stored.MessagesSnapshot), "https://signed.example/generated.wav")
	var references []model.PlaygroundRecordAsset
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).Find(&references).Error)
	require.Len(t, references, 1)
	require.Equal(t, "ast_controller_generated_audio", references[0].AssetID)
}

func TestSaveAndClearPlaygroundRecordPreservesDocumentAttachmentReferences(t *testing.T) {
	setupPlaygroundControllerDB(t, 215)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.Asset{
		PublicId: "ast_controller_pdf_attachment", UserId: 215, AssetType: "Document",
		Status: model.AssetStatusActive, SourceStatus: model.AssetSourceStatusAvailable,
		StorageBackend: "gcs", StorageBucket: "bucket", ObjectKey: "report.pdf", ObjectGeneration: 1,
		ContentType: "application/pdf", SizeBytes: 1, SourceExpiresAt: now + 3600, CreatedAt: now, UpdatedAt: now,
	}).Error)
	saveBody := `{
		"record_id":"550e8400-e29b-41d4-a716-446655440000",
		"conversation_id":"550e8400-e29b-41d4-a716-446655440001",
		"user_message":{"key":"u","from":"user","versions":[{"id":"uv","content":"hello","attachments":[{"kind":"pdf","filename":"report.pdf","mediaType":"application/pdf","assetId":"ast_controller_pdf_attachment","file_url":"https://signed.example/report.pdf"}]}]},
		"request_messages":[{"role":"user","content":[{"type":"input_file","file_url":"https://signed.example/report.pdf"}]}],
		"assistant_message":{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"},
		"reasoning_content":"","input_text":"hello","output_text":"world","model_name":"gpt-test","group_name":"plg",
		"parameters":{},"status":"complete","error_code":"","error_message":"","relay_request_id":"","prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"latency_ms":1,
		"messages_snapshot":[{"key":"u","from":"user","versions":[{"id":"uv","content":"hello","attachments":[{"kind":"pdf","filename":"report.pdf","mediaType":"application/pdf","assetId":"ast_controller_pdf_attachment","file_url":"https://signed.example/report.pdf"}]}]},{"key":"a","from":"assistant","versions":[{"id":"av","content":"world"}],"status":"complete"}],
		"client_completed_at":1000
	}`
	c, recorder := playgroundRecordTestContext(t, 215, http.MethodPost, saveBody)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var stored model.PlaygroundRecord
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).First(&stored).Error)
	// The durable attachment reference is represented by assetId in the
	// message snapshot; the signed file URL is intentionally removed. The
	// request copy below uses an asset:// URI so it can be replayed safely.
	require.Contains(t, string(stored.UserMessage), "ast_controller_pdf_attachment")
	require.NotContains(t, string(stored.UserMessage), "https://signed.example/report.pdf")
	require.Contains(t, string(stored.RequestMessages), "asset://ast_controller_pdf_attachment")
	require.NotContains(t, string(stored.RequestMessages), "https://signed.example/report.pdf")
	var references []model.PlaygroundRecordAsset
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).Find(&references).Error)
	require.Len(t, references, 1)
	require.Equal(t, "ast_controller_pdf_attachment", references[0].AssetID)
	var storedAsset model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", references[0].AssetID).First(&storedAsset).Error)
	require.Equal(t, "Document", storedAsset.AssetType)

	clearBody := `{"record_id":"550e8400-e29b-41d4-a716-446655440002","conversation_id":"550e8400-e29b-41d4-a716-446655440001","client_completed_at":2000}`
	c, recorder = playgroundRecordTestContext(t, 215, http.MethodPost, clearBody)

	ClearPlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NoError(t, model.DB.Where("record_id = ?", playgroundRecordID).Find(&references).Error)
	require.Empty(t, references)
}

func TestSavePlaygroundRecordRejectsInvalidIdentityAndStatus(t *testing.T) {
	setupPlaygroundControllerDB(t, 205)
	tests := []struct {
		name string
		body string
	}{
		{
			name: "record uuid",
			body: strings.Replace(validPlaygroundRecordBody(), playgroundRecordID, "not-a-uuid", 1),
		},
		{
			name: "conversation uuid",
			body: strings.Replace(validPlaygroundRecordBody(), playgroundConversationID, "not-a-uuid", 1),
		},
		{
			name: "status",
			body: strings.Replace(
				validPlaygroundRecordBody(),
				"\n\t\t\"status\":\"complete\",\n\t\t\"error_code\"",
				"\n\t\t\"status\":\"cleared\",\n\t\t\"error_code\"",
				1,
			),
		},
		{
			name: "completion time",
			body: strings.Replace(validPlaygroundRecordBody(), `"client_completed_at":1000`, `"client_completed_at":0`, 1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := playgroundRecordTestContext(t, 205, http.MethodPost, test.body)
			SavePlaygroundRecord(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestSavePlaygroundRecordRejectsExcessiveJSONDepth(t *testing.T) {
	setupPlaygroundControllerDB(t, 206)
	nested := any("leaf")
	for range 33 {
		nested = map[string]any{"nested": nested}
	}
	parameters, err := common.Marshal(nested)
	require.NoError(t, err)
	body := strings.Replace(
		validPlaygroundRecordBody(),
		`{"temperature":0.7}`,
		string(parameters),
		1,
	)
	c, recorder := playgroundRecordTestContext(t, 206, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSavePlaygroundRecordRejectsOversizedBody(t *testing.T) {
	setupPlaygroundControllerDB(t, 207)
	body := strings.Replace(
		validPlaygroundRecordBody(),
		`"error_message":""`,
		`"error_message":"`+strings.Repeat("x", (16<<20)+1)+`"`,
		1,
	)
	c, recorder := playgroundRecordTestContext(t, 207, http.MethodPost, body)

	SavePlaygroundRecord(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetCurrentPlaygroundRecordUsesAuthenticatedUser(t *testing.T) {
	setupPlaygroundControllerDB(t, 208, 209)
	record := &model.PlaygroundRecord{
		UserID:            208,
		RecordID:          playgroundRecordID,
		RecordType:        model.PlaygroundRecordTypeTurn,
		ConversationID:    playgroundConversationID,
		Status:            model.PlaygroundStatusComplete,
		MessagesSnapshot:  `[{"key":"owned-by-208"}]`,
		ClientCompletedAt: 1000,
	}
	require.NoError(t, model.SavePlaygroundRecord(record))
	c, recorder := playgroundRecordTestContext(t, 209, http.MethodGet, "")

	GetCurrentPlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":null}`, recorder.Body.String())
}

func TestClearPlaygroundRecordRemovesCurrentAndKeepsHistory(t *testing.T) {
	setupPlaygroundControllerDB(t, 210)
	record := &model.PlaygroundRecord{
		UserID:            210,
		RecordID:          playgroundRecordID,
		RecordType:        model.PlaygroundRecordTypeTurn,
		ConversationID:    playgroundConversationID,
		Status:            model.PlaygroundStatusComplete,
		OutputText:        "durable output",
		MessagesSnapshot:  `[{"key":"durable"}]`,
		ClientCompletedAt: 1000,
	}
	require.NoError(t, model.SavePlaygroundRecord(record))
	clearBody := `{"record_id":"550e8400-e29b-41d4-a716-446655440002","conversation_id":"550e8400-e29b-41d4-a716-446655440001","client_completed_at":2000}`
	c, recorder := playgroundRecordTestContext(t, 210, http.MethodPost, clearBody)

	ClearPlaygroundRecord(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	current, err := model.GetCurrentPlaygroundRecord(210)
	require.NoError(t, err)
	require.Nil(t, current)
	var stored model.PlaygroundRecord
	require.NoError(t, model.DB.Where("user_id = ? AND record_id = ?", 210, playgroundRecordID).First(&stored).Error)
	require.Equal(t, "durable output", string(stored.OutputText))
	require.NotEmpty(t, stored.MessagesSnapshot)
}

func TestExportPlaygroundRecords(t *testing.T) {
	setupPlaygroundControllerDB(t, 211, 212)
	recordA := &model.PlaygroundRecord{
		UserID:            211,
		RecordID:          "record-a",
		RecordType:        model.PlaygroundRecordTypeTurn,
		ConversationID:    "conversation-a",
		ConversationName:  "Export demo",
		UserMessage:       `{"content":"=SUM(1,1)"}`,
		RequestMessages:   `[{"role":"user","content":"hello"}]`,
		Parameters:        `{"temperature":0.7}`,
		OutputText:        "owned output",
		Status:            model.PlaygroundStatusComplete,
		ClientCompletedAt: 1000,
	}
	recordB := &model.PlaygroundRecord{
		UserID:            212,
		RecordID:          "record-b",
		RecordType:        model.PlaygroundRecordTypeTurn,
		ConversationID:    "conversation-b",
		OutputText:        "other output",
		Status:            model.PlaygroundStatusError,
		ClientCompletedAt: 2000,
	}
	require.NoError(t, model.SavePlaygroundRecord(recordA))
	require.NoError(t, model.SavePlaygroundRecord(recordB))
	require.NoError(t, model.DB.Create(&model.PlaygroundRecord{
		UserID:            211,
		RecordID:          "record-clear",
		RecordType:        model.PlaygroundRecordTypeClear,
		ConversationID:    "conversation-a",
		Status:            model.PlaygroundStatusCleared,
		ClientCompletedAt: 3000,
	}).Error)

	t.Run("filters by user and returns a download", func(t *testing.T) {
		c, recorder := playgroundExportTestContext(t, 1, "user_id=211")

		ExportPlaygroundRecords(c)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, playgroundExportMimeType, recorder.Header().Get("Content-Type"))
		require.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment; filename=playground-records-211-")
		require.Contains(t, recorder.Header().Get("Content-Disposition"), ".xlsx")
		workbook := openPlaygroundWorkbook(t, recorder.Body.Bytes())
		rows, err := workbook.GetRows(playgroundExportSheetName)
		require.NoError(t, err)
		require.Len(t, rows, 3)
		require.Equal(t, expectedPlaygroundExportHeaders, rows[0])
		require.Equal(t, []string{"record-a", "record-clear"}, []string{rows[1][2], rows[2][2]})
		require.Equal(t, "Export demo", rows[1][5])
		require.Equal(t, `{"content":"=SUM(1,1)"}`, rows[1][6])
		require.Equal(t, `[{"role":"user","content":"hello"}]`, rows[1][7])
		require.Equal(t, "owned output", rows[1][11])
		require.Equal(t, model.PlaygroundRecordTypeClear, rows[2][3])
	})

	t.Run("without a filter returns every user", func(t *testing.T) {
		c, recorder := playgroundExportTestContext(t, 1, "")

		ExportPlaygroundRecords(c)

		require.Equal(t, http.StatusOK, recorder.Code)
		workbook := openPlaygroundWorkbook(t, recorder.Body.Bytes())
		rows, err := workbook.GetRows(playgroundExportSheetName)
		require.NoError(t, err)
		require.Equal(t, []string{"record-a", "record-clear", "record-b"}, []string{rows[1][2], rows[2][2], rows[3][2]})
	})

	t.Run("empty filter result is a workbook with headers", func(t *testing.T) {
		c, recorder := playgroundExportTestContext(t, 1, "user_id=999999")

		ExportPlaygroundRecords(c)

		require.Equal(t, http.StatusOK, recorder.Code)
		workbook := openPlaygroundWorkbook(t, recorder.Body.Bytes())
		rows, err := workbook.GetRows(playgroundExportSheetName)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, expectedPlaygroundExportHeaders, rows[0])
	})

	t.Run("explicit JSON format remains compatible", func(t *testing.T) {
		c, recorder := playgroundExportTestContext(t, 1, "user_id=211&format=json")

		ExportPlaygroundRecords(c)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
		var exported []model.PlaygroundRecord
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &exported))
		require.Equal(t, []string{"record-a", "record-clear"}, exportedPlaygroundRecordIDs(exported))
	})

	t.Run("rejects unsupported format", func(t *testing.T) {
		c, recorder := playgroundExportTestContext(t, 1, "format=csv")

		ExportPlaygroundRecords(c)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), "success")
		require.NotContains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	})

	for _, query := range []string{"user_id=not-a-number", "user_id=", "user_id=%20", "user_id=0", "user_id=-1"} {
		t.Run("rejects invalid user filter "+query, func(t *testing.T) {
			c, recorder := playgroundExportTestContext(t, 1, query)

			ExportPlaygroundRecords(c)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Contains(t, recorder.Body.String(), "success")
			require.NotContains(t, recorder.Header().Get("Content-Disposition"), "attachment")
		})
	}
}

func openPlaygroundWorkbook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	require.True(t, bytes.HasPrefix(data, []byte("PK")), "XLSX should be a ZIP archive")
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { _ = workbook.Close() })
	return workbook
}

var expectedPlaygroundExportHeaders = []string{
	"id",
	"user_id",
	"record_id",
	"record_type",
	"conversation_id",
	"conversation_name",
	"user_message",
	"request_messages",
	"assistant_message",
	"reasoning_content",
	"input_text",
	"output_text",
	"model_name",
	"group_name",
	"parameters",
	"status",
	"error_code",
	"error_message",
	"relay_request_id",
	"prompt_tokens",
	"completion_tokens",
	"total_tokens",
	"latency_ms",
	"messages_snapshot",
	"is_latest",
	"is_current",
	"client_completed_at",
	"created_at",
	"updated_at",
	"client_completed_at_utc",
	"created_at_utc",
	"updated_at_utc",
}

func TestBuildPlaygroundRecordsWorkbookPreservesLongText(t *testing.T) {
	original := strings.Repeat("长", playgroundExcelCellLimit+11)
	data, err := buildPlaygroundRecordsWorkbook([]model.PlaygroundRecord{{
		UserID:      213,
		RecordID:    "record-long",
		RecordType:  model.PlaygroundRecordTypeTurn,
		UserMessage: model.PlaygroundLargeText(original),
	}})
	require.NoError(t, err)

	workbook := openPlaygroundWorkbook(t, data)
	mainRows, err := workbook.GetRows(playgroundExportSheetName)
	require.NoError(t, err)
	require.Contains(t, mainRows[1][6], playgroundExportTextSheetName)

	textRows, err := workbook.GetRows(playgroundExportTextSheetName)
	require.NoError(t, err)
	require.Greater(t, len(textRows), 1)
	var rebuilt strings.Builder
	for _, row := range textRows[1:] {
		require.Equal(t, "record-long", row[1])
		require.Equal(t, "user_message", row[2])
		rebuilt.WriteString(row[5])
	}
	require.Equal(t, original, rebuilt.String())

	emojiParts := splitPlaygroundExportText(strings.Repeat("😀", playgroundExcelCellLimit/2+1), playgroundExcelCellLimit)
	require.Len(t, emojiParts, 2)
	for _, part := range emojiParts {
		require.LessOrEqual(t, playgroundExportUTF16Length(part), playgroundExcelCellLimit)
	}
}

func TestParsePlaygroundExportFormat(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		present bool
		want    string
		wantErr bool
	}{
		{name: "missing defaults to xlsx", want: "xlsx"},
		{name: "explicit xlsx", raw: "XLSX", present: true, want: "xlsx"},
		{name: "explicit json", raw: " json ", present: true, want: "json"},
		{name: "blank rejected", raw: " ", present: true, wantErr: true},
		{name: "unknown rejected", raw: "csv", present: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePlaygroundExportFormat(test.raw, test.present)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func exportedPlaygroundRecordIDs(records []model.PlaygroundRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.RecordID)
	}
	return ids
}

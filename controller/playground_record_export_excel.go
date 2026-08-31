package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const (
	playgroundExportSheetName       = "Playground Records"
	playgroundExportTextSheetName   = "Playground Record Text"
	playgroundExportReadmeSheetName = "README"
	playgroundExportMimeType        = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	playgroundExcelCellLimit        = 32767
)

var playgroundExportHeaders = []string{
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

var playgroundExportTextHeaders = []string{
	"user_id",
	"record_id",
	"field",
	"chunk_index",
	"chunk_count",
	"value",
}

func parsePlaygroundExportFormat(raw string, present bool) (string, error) {
	if !present {
		return "xlsx", nil
	}
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		return "", fmt.Errorf("format must not be empty")
	}
	switch format {
	case "xlsx", "json":
		return format, nil
	default:
		return "", fmt.Errorf("unsupported format %q", raw)
	}
}

func exportPlaygroundRecordsJSON(
	c *gin.Context,
	records []model.PlaygroundRecord,
	userID *int,
) {
	data, err := common.Marshal(records)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := playgroundExportFilename(userID, "json")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

func playgroundExportFilename(userID *int, extension string) string {
	filename := "playground-records"
	if userID != nil {
		filename += fmt.Sprintf("-%d", *userID)
	}
	return filename + "-" + time.Now().UTC().Format("20060102T150405Z") + "." + extension
}

type playgroundExportTextChunk struct {
	userID     int
	recordID   string
	field      string
	chunkIndex int
	chunkCount int
	value      string
}

func buildPlaygroundRecordsWorkbook(records []model.PlaygroundRecord) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()

	if err := file.SetSheetName("Sheet1", playgroundExportSheetName); err != nil {
		return nil, err
	}
	if _, err := file.NewSheet(playgroundExportTextSheetName); err != nil {
		return nil, err
	}
	if _, err := file.NewSheet(playgroundExportReadmeSheetName); err != nil {
		return nil, err
	}

	if err := writePlaygroundExportHeaders(file, playgroundExportSheetName, playgroundExportHeaders); err != nil {
		return nil, err
	}
	if err := writePlaygroundExportHeaders(file, playgroundExportTextSheetName, playgroundExportTextHeaders); err != nil {
		return nil, err
	}
	styles, err := newPlaygroundExportStyles(file)
	if err != nil {
		return nil, err
	}
	if err := applyPlaygroundExportHeaderStyle(file, playgroundExportSheetName, playgroundExportHeaders, styles.header); err != nil {
		return nil, err
	}
	if err := applyPlaygroundExportHeaderStyle(file, playgroundExportTextSheetName, playgroundExportTextHeaders, styles.header); err != nil {
		return nil, err
	}

	chunks := make([]playgroundExportTextChunk, 0)
	for index, record := range records {
		row := index + 2
		values := playgroundExportRecordValues(record)
		for column, value := range values {
			cell, err := excelize.CoordinatesToCellName(column+1, row)
			if err != nil {
				return nil, err
			}
			if text, ok := value.(string); ok {
				if text == "" {
					continue
				}
				if playgroundExportUTF16Length(text) > playgroundExcelCellLimit {
					parts := splitPlaygroundExportText(text, playgroundExcelCellLimit)
					for chunkIndex, part := range parts {
						chunks = append(chunks, playgroundExportTextChunk{
							userID:     record.UserID,
							recordID:   record.RecordID,
							field:      playgroundExportHeaders[column],
							chunkIndex: chunkIndex + 1,
							chunkCount: len(parts),
							value:      part,
						})
					}
					marker := fmt.Sprintf(
						"See %s: user_id=%d, record_id=%s, field=%s",
						playgroundExportTextSheetName,
						record.UserID,
						record.RecordID,
						playgroundExportHeaders[column],
					)
					if err := file.SetCellStr(playgroundExportSheetName, cell, marker); err != nil {
						return nil, err
					}
					continue
				}
				if err := file.SetCellStr(playgroundExportSheetName, cell, text); err != nil {
					return nil, err
				}
				continue
			}
			if err := file.SetCellValue(playgroundExportSheetName, cell, value); err != nil {
				return nil, err
			}
		}
	}

	for index, chunk := range chunks {
		row := index + 2
		values := []any{
			chunk.userID,
			chunk.recordID,
			chunk.field,
			chunk.chunkIndex,
			chunk.chunkCount,
			chunk.value,
		}
		for column, value := range values {
			cell, err := excelize.CoordinatesToCellName(column+1, row)
			if err != nil {
				return nil, err
			}
			if text, ok := value.(string); ok {
				if text == "" {
					continue
				}
				if err := file.SetCellStr(playgroundExportTextSheetName, cell, text); err != nil {
					return nil, err
				}
			} else if err := file.SetCellValue(playgroundExportTextSheetName, cell, value); err != nil {
				return nil, err
			}
		}
	}

	if err := writePlaygroundExportReadme(file, len(chunks) > 0, styles.title); err != nil {
		return nil, err
	}
	if err := configurePlaygroundExportSheet(file, playgroundExportSheetName, len(records), styles.body); err != nil {
		return nil, err
	}
	if err := configurePlaygroundExportSheet(file, playgroundExportTextSheetName, len(chunks), styles.body); err != nil {
		return nil, err
	}
	file.SetActiveSheet(0)

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writePlaygroundExportHeaders(file *excelize.File, sheet string, headers []string) error {
	for column, header := range headers {
		cell, err := excelize.CoordinatesToCellName(column+1, 1)
		if err != nil {
			return err
		}
		if err := file.SetCellStr(sheet, cell, header); err != nil {
			return err
		}
	}
	return nil
}

type playgroundExportStyles struct {
	header int
	body   int
	title  int
}

func newPlaygroundExportStyles(file *excelize.File) (playgroundExportStyles, error) {
	header, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F4E78"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return playgroundExportStyles{}, err
	}
	body, err := file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	if err != nil {
		return playgroundExportStyles{}, err
	}
	title, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "1F4E78"},
	})
	if err != nil {
		return playgroundExportStyles{}, err
	}
	return playgroundExportStyles{header: header, body: body, title: title}, nil
}

func applyPlaygroundExportHeaderStyle(
	file *excelize.File,
	sheet string,
	headers []string,
	styleID int,
) error {
	lastCell, err := excelize.CoordinatesToCellName(len(headers), 1)
	if err != nil {
		return err
	}
	if err := file.SetCellStyle(sheet, "A1", lastCell, styleID); err != nil {
		return err
	}
	return file.SetRowHeight(sheet, 1, 28)
}

func writePlaygroundExportReadme(file *excelize.File, hasChunks bool, titleStyle int) error {
	lines := []string{
		"Playground record export",
		"Rows are ordered by user_id, client_completed_at, record_id, and database id.",
		"JSON fields are stored as text so their original content can be copied out.",
		"Excel limits one cell to 32767 Unicode characters.",
	}
	if hasChunks {
		lines = append(lines, "Long values are referenced in Playground Record Text; concatenate chunks by chunk_index.")
	} else {
		lines = append(lines, "This export contains no values longer than the Excel cell limit.")
	}
	for row, line := range lines {
		cell, err := excelize.CoordinatesToCellName(1, row+1)
		if err != nil {
			return err
		}
		if err := file.SetCellStr(playgroundExportReadmeSheetName, cell, line); err != nil {
			return err
		}
	}
	if err := file.SetCellStyle(playgroundExportReadmeSheetName, "A1", "A1", titleStyle); err != nil {
		return err
	}
	if err := file.SetColWidth(playgroundExportReadmeSheetName, "A", "A", 100); err != nil {
		return err
	}
	return nil
}

func configurePlaygroundExportSheet(file *excelize.File, sheet string, dataRows int, bodyStyle int) error {
	lastRow := dataRows + 1
	columnCount := len(playgroundExportHeaders)
	lastCell, err := excelize.CoordinatesToCellName(columnCount, lastRow)
	if sheet == playgroundExportTextSheetName {
		columnCount = len(playgroundExportTextHeaders)
		lastCell, err = excelize.CoordinatesToCellName(columnCount, lastRow)
	}
	if err != nil {
		return err
	}
	lastColumn, err := excelize.ColumnNumberToName(columnCount)
	if err != nil {
		return err
	}
	if err := file.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return err
	}
	showGridLines := false
	zoomScale := float64(90)
	if err := file.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines, ZoomScale: &zoomScale}); err != nil {
		return err
	}
	if err := file.AutoFilter(sheet, "A1:"+lastColumn+strconv.Itoa(lastRow), nil); err != nil {
		return err
	}
	if dataRows > 0 {
		if err := file.SetCellStyle(sheet, "A2", lastCell, bodyStyle); err != nil {
			return err
		}
	}
	if err := file.SetColWidth(sheet, "A", lastColumn, 18); err != nil {
		return err
	}
	if sheet == playgroundExportSheetName {
		if err := file.SetColWidth(sheet, "G", "L", 34); err != nil {
			return err
		}
		if err := file.SetColWidth(sheet, "X", "AF", 28); err != nil {
			return err
		}
	} else if err := file.SetColWidth(sheet, "F", "F", 60); err != nil {
		return err
	}
	return nil
}

func playgroundExportRecordValues(record model.PlaygroundRecord) []any {
	return []any{
		strconv.FormatInt(record.ID, 10),
		strconv.Itoa(record.UserID),
		record.RecordID,
		record.RecordType,
		record.ConversationID,
		record.ConversationName,
		string(record.UserMessage),
		string(record.RequestMessages),
		string(record.AssistantMessage),
		string(record.ReasoningContent),
		string(record.InputText),
		string(record.OutputText),
		record.ModelName,
		record.GroupName,
		string(record.Parameters),
		record.Status,
		record.ErrorCode,
		string(record.ErrorMessage),
		record.RelayRequestID,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.LatencyMS,
		string(record.MessagesSnapshot),
		record.IsLatest,
		record.IsCurrent,
		record.ClientCompletedAt,
		formatPlaygroundExportTime(record.CreatedAt),
		formatPlaygroundExportTime(record.UpdatedAt),
		formatPlaygroundExportUnixMillis(record.ClientCompletedAt),
		formatPlaygroundExportTime(record.CreatedAt),
		formatPlaygroundExportTime(record.UpdatedAt),
	}
}

func formatPlaygroundExportTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatPlaygroundExportUnixMillis(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.UnixMilli(value).UTC().Format(time.RFC3339Nano)
}

func splitPlaygroundExportText(value string, limit int) []string {
	if limit <= 0 || playgroundExportUTF16Length(value) <= limit {
		return []string{value}
	}
	parts := make([]string, 0)
	start := 0
	units := 0
	for index, r := range value {
		runeUnits := utf16.RuneLen(r)
		if units > 0 && units+runeUnits > limit {
			parts = append(parts, value[start:index])
			start = index
			units = 0
		}
		units += runeUnits
	}
	parts = append(parts, value[start:])
	return parts
}

func playgroundExportUTF16Length(value string) int {
	length := 0
	for _, r := range value {
		length += utf16.RuneLen(r)
	}
	return length
}

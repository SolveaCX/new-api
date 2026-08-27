package dto

// PlaygroundAttachmentPreviewResponse contains only public attachment
// metadata and a short-lived signed URL. Storage bucket/object details remain
// server-side.
type PlaygroundAttachmentPreviewResponse struct {
	AssetID     string `json:"asset_id"`
	AssetType   string `json:"asset_type"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	PreviewURL  string `json:"preview_url"`
	ExpiresAt   int64  `json:"expires_at"`
}

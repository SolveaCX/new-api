package service

import (
	"regexp"
	"strings"
)

// Data tools whitelabel.
//
// The upstream tool catalog exposes a vendor brand in tool ids
// ("blockrun.audio.speech"), provider labels ("blockrun:audio"), the platform
// filter ("BlockRun") and occasionally in descriptions. Customers only ever
// see the Flatkey brand: ids and platform names are rewritten on the way out
// and mapped back on the way in (inspect / run), so existing upstream ids keep
// working while nothing vendor-specific reaches the client.

const (
	upstreamDataToolVendor = "blockrun"
	publicDataToolVendor   = "flatkey"
)

var dataToolVendorWordPattern = regexp.MustCompile(`(?i)` + upstreamDataToolVendor)

// UpstreamDataToolIDCandidates returns the upstream ids to try for a
// client-facing id, most likely first. Ids come in several shapes
// ("blockrun.audio.speech", "gateway:monid:blockrun.ai:/api/..."), so the
// public form is produced by a generic rewrite and reversed by trying the
// rewritten candidate before the id as given.
func UpstreamDataToolIDCandidates(id string) []string {
	id = strings.TrimSpace(id)
	mapped := id
	if strings.HasPrefix(mapped, publicDataToolVendor+".") {
		mapped = upstreamDataToolVendor + "." + strings.TrimPrefix(mapped, publicDataToolVendor+".")
	}
	mapped = strings.ReplaceAll(mapped, publicDataToolVendor+".ai", upstreamDataToolVendor+".ai")
	mapped = strings.ReplaceAll(mapped, ":"+publicDataToolVendor+":", ":"+upstreamDataToolVendor+":")
	if mapped == id {
		return []string{id}
	}
	return []string{mapped, id}
}

// UpstreamDataToolID maps a client-facing tool id back to the most likely
// upstream id (first candidate).
func UpstreamDataToolID(id string) string {
	return UpstreamDataToolIDCandidates(id)[0]
}

// PublicDataToolID maps an upstream tool id to the client-facing id.
func PublicDataToolID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, upstreamDataToolVendor+".") {
		id = publicDataToolVendor + "." + strings.TrimPrefix(id, upstreamDataToolVendor+".")
	}
	return dataToolVendorWordPattern.ReplaceAllString(id, publicDataToolVendor)
}

// UpstreamDataToolPlatform maps the client-facing platform filter back to the
// upstream platform name.
func UpstreamDataToolPlatform(platform string) string {
	if strings.EqualFold(strings.TrimSpace(platform), publicDataToolVendor) {
		return "BlockRun"
	}
	return platform
}

func publicDataToolPlatform(platform string) string {
	if strings.EqualFold(strings.TrimSpace(platform), upstreamDataToolVendor) {
		return publicDataToolVendor
	}
	return platform
}

func publicDataToolProvider(provider string) string {
	if strings.HasPrefix(strings.ToLower(provider), upstreamDataToolVendor+":") {
		return publicDataToolVendor + ":" + provider[len(upstreamDataToolVendor)+1:]
	}
	return replaceDataToolVendorText(provider)
}

func replaceDataToolVendorText(text string) string {
	if text == "" {
		return text
	}
	return dataToolVendorWordPattern.ReplaceAllString(text, publicDataToolVendor)
}

func whitelabelDataToolSummary(tool *DataToolSummary) {
	tool.ID = PublicDataToolID(tool.ID)
	tool.Name = replaceDataToolVendorText(tool.Name)
	tool.Provider = publicDataToolProvider(tool.Provider)
	tool.Platform = publicDataToolPlatform(tool.Platform)
	tool.Description = replaceDataToolVendorText(tool.Description)
	for i := range tool.Categories {
		tool.Categories[i] = replaceDataToolVendorText(tool.Categories[i])
	}
}

func whitelabelDataToolList(list *DataToolList) {
	if list == nil {
		return
	}
	for i := range list.Tools {
		whitelabelDataToolSummary(&list.Tools[i])
	}
	for i := range list.Platforms {
		list.Platforms[i].Platform = publicDataToolPlatform(list.Platforms[i].Platform)
	}
}

func whitelabelDataToolInspection(inspection *DataToolInspection) {
	if inspection == nil {
		return
	}
	inspection.ID = PublicDataToolID(inspection.ID)
	inspection.Name = replaceDataToolVendorText(inspection.Name)
	inspection.Provider = publicDataToolProvider(inspection.Provider)
	inspection.Description = replaceDataToolVendorText(inspection.Description)
	for key, field := range inspection.Input.Properties {
		field.Description = replaceDataToolVendorText(field.Description)
		inspection.Input.Properties[key] = field
	}
}

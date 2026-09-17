package compute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// RFQDraft is the structured form the "paste a paragraph, auto-fill" feature
// produces from free text. Every field is optional; Filled/Missing tell the
// UI which fields to mark as AI-filled versus needing confirmation.
type RFQDraft struct {
	GpuModel       string   `json:"gpu_model,omitempty"`
	GpusPerNode    int      `json:"gpus_per_node,omitempty"`
	Nodes          int      `json:"nodes,omitempty"`
	TermMonths     int      `json:"term_months,omitempty"`
	StartDate      string   `json:"start_date,omitempty"`
	PriceCeiling   float64  `json:"price_ceiling,omitempty"`
	TargetPriceMin float64  `json:"target_price_min,omitempty"`
	TargetPriceMax float64  `json:"target_price_max,omitempty"`
	Regions        []string `json:"regions,omitempty"`
	Delivery       string   `json:"delivery,omitempty"`
	Interconnect   string   `json:"interconnect,omitempty"`
	StorageTB      float64  `json:"storage_tb,omitempty"`
	Compliance     []string `json:"compliance,omitempty"`
	Notes          string   `json:"notes,omitempty"`

	Source  string   `json:"source"`
	Filled  []string `json:"filled"`
	Missing []string `json:"missing"`
}

var rfqRequiredFields = []string{"gpu_model", "nodes", "term_months", "start_date", "price_ceiling", "regions"}

// ParseRFQText turns a free-text description into an RFQDraft. It tries the
// configured monitoring AI endpoint first (same OpenAI-compatible settings the
// alert summariser uses) and always falls back to the built-in heuristic
// parser, so the feature works without any key configured.
func ParseRFQText(ctx context.Context, text string) *RFQDraft {
	text = strings.TrimSpace(text)
	if text == "" {
		return &RFQDraft{Source: "heuristic", Filled: []string{}, Missing: append([]string{}, rfqRequiredFields...)}
	}
	if d, err := parseRFQWithAI(ctx, text); err == nil && d != nil {
		d.Source = "ai"
		finalizeRFQDraft(d)
		return d
	}
	d := ParseRFQHeuristic(text)
	d.Source = "heuristic"
	finalizeRFQDraft(d)
	return d
}

func finalizeRFQDraft(d *RFQDraft) {
	// Always emit JSON arrays (never null) so clients can rely on .length.
	d.Filled = make([]string, 0, 11)
	d.Missing = make([]string, 0, len(rfqRequiredFields))
	has := map[string]bool{
		"gpu_model":     d.GpuModel != "",
		"gpus_per_node": d.GpusPerNode > 0,
		"nodes":         d.Nodes > 0,
		"term_months":   d.TermMonths > 0,
		"start_date":    d.StartDate != "",
		"price_ceiling": d.PriceCeiling > 0,
		"regions":       len(d.Regions) > 0,
		"delivery":      d.Delivery != "",
		"interconnect":  d.Interconnect != "",
		"storage_tb":    d.StorageTB > 0,
		"compliance":    len(d.Compliance) > 0,
	}
	for _, f := range []string{"gpu_model", "gpus_per_node", "nodes", "term_months", "start_date", "price_ceiling", "regions", "delivery", "interconnect", "storage_tb", "compliance"} {
		if has[f] {
			d.Filled = append(d.Filled, f)
		}
	}
	for _, f := range rfqRequiredFields {
		if !has[f] {
			d.Missing = append(d.Missing, f)
		}
	}
	if d.PriceCeiling == 0 && d.TargetPriceMax > 0 {
		d.PriceCeiling = d.TargetPriceMax
	}
}

var (
	reGpu        = regexp.MustCompile(`(?i)\b(B300|B200|GB300|GB200|H200|H100|H800|A100|A800|L40S|L40|RTX ?5090|RTX ?4090|4090|5090|M3 Ultra|M4 Max)\b`)
	reNodesZh    = regexp.MustCompile(`(\d+)\s*(台|个节点|节点|机)`)
	reNodesEn    = regexp.MustCompile(`(?i)(\d+)\s*(nodes?|servers?|machines?|boxes)`)
	reGpusZh     = regexp.MustCompile(`每台\s*(\d+)\s*卡|(\d+)\s*卡\s*/\s*台|单机\s*(\d+)\s*卡`)
	reGpusEn     = regexp.MustCompile(`(?i)(\d+)\s*(?:x\s*)?(?:gpus?|cards?)\s*(?:per|/)\s*(?:node|server|box)|(\d+)[x×]\s*(?:B300|B200|H200|H100|A100)`)
	reGpusTotal  = regexp.MustCompile(`(?i)(\d+)\s*(?:张|块|个)?\s*(?:gpus?|卡)\b`)
	reMonthsZh   = regexp.MustCompile(`(\d+)\s*个月`)
	reMonthsEn   = regexp.MustCompile(`(?i)(\d+)\s*months?`)
	reYearsZh    = regexp.MustCompile(`(一|二|两|三|1|2|3)\s*年`)
	reYearsEn    = regexp.MustCompile(`(?i)(\d+|a|one|two)\s*years?`)
	reISODate    = regexp.MustCompile(`(20\d{2})[-/.](\d{1,2})[-/.](\d{1,2})`)
	reDateZh     = regexp.MustCompile(`(\d{1,2})\s*月\s*(\d{1,2})\s*[号日]`)
	reDateEn     = regexp.MustCompile(`(?i)\b(jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\.?\s+(\d{1,2})`)
	rePriceRange = regexp.MustCompile(`\$?\s*(\d+(?:\.\d+)?)\s*(?:到|至|-|–|—|~|to)\s*\$?\s*(\d+(?:\.\d+)?)\s*(?:美[金元]|usd|\$|dollars?)?`)
	rePriceOne   = regexp.MustCompile(`(?i)(?:\$|usd\s*)(\d+(?:\.\d+)?)|(\d+(?:\.\d+)?)\s*(?:美[金元]|usd|dollars?)`)
	reStorage    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(PB|TB)`)
	reIB         = regexp.MustCompile(`(?i)(?:IB|InfiniBand|RoCE)\s*(\d{3})\s*G`)
	reSLA        = regexp.MustCompile(`(?i)SLA\s*(\d{2}(?:\.\d+)?)`)
	reTier       = regexp.MustCompile(`(?i)Tier\s*(3|4|III|IV)`)
)

var regionKeywords = []struct {
	code string
	keys []string
}{
	{"JP", []string{"日本", "东京", "大阪", "japan", "tokyo", "osaka"}},
	{"SG", []string{"新加坡", "singapore"}},
	{"KR", []string{"韩国", "首尔", "korea", "seoul"}},
	{"US-WEST", []string{"美西", "us-west", "us west", "california", "oregon", "silicon valley"}},
	{"US-EAST", []string{"美东", "us-east", "us east", "virginia"}},
	{"EU", []string{"欧洲", "europe", "eu ", "frankfurt", "amsterdam", "germany", "netherlands"}},
	{"HK", []string{"香港", "hong kong"}},
	{"TW", []string{"台湾", "taiwan"}},
	{"CN", []string{"中国大陆", "国内", "mainland"}},
	{"IN", []string{"印度", "india", "mumbai"}},
	{"AU", []string{"澳洲", "澳大利亚", "australia", "sydney"}},
	{"ME", []string{"中东", "迪拜", "middle east", "dubai", "saudi"}},
}

// ParseRFQHeuristic extracts RFQ fields with regexes; zh/en friendly.
func ParseRFQHeuristic(text string) *RFQDraft {
	d := &RFQDraft{}
	lower := strings.ToLower(text)

	if m := reGpu.FindStringSubmatch(text); m != nil {
		g := strings.ToUpper(strings.ReplaceAll(m[1], " ", ""))
		switch g {
		case "4090":
			g = "RTX4090"
		case "5090":
			g = "RTX5090"
		case "M3ULTRA":
			g = "M3 Ultra"
		case "M4MAX":
			g = "M4 Max"
		}
		d.GpuModel = strings.Replace(strings.Replace(g, "RTX4090", "RTX 4090", 1), "RTX5090", "RTX 5090", 1)
	}
	if m := reNodesZh.FindStringSubmatch(text); m != nil {
		d.Nodes, _ = strconv.Atoi(m[1])
	} else if m := reNodesEn.FindStringSubmatch(text); m != nil {
		d.Nodes, _ = strconv.Atoi(m[1])
	}
	if m := reGpusZh.FindStringSubmatch(text); m != nil {
		for _, g := range m[1:] {
			if g != "" {
				d.GpusPerNode, _ = strconv.Atoi(g)
			}
		}
	} else if m := reGpusEn.FindStringSubmatch(text); m != nil {
		for _, g := range m[1:] {
			if g != "" {
				d.GpusPerNode, _ = strconv.Atoi(g)
			}
		}
	}
	if d.Nodes == 0 && d.GpusPerNode == 0 {
		if m := reGpusTotal.FindStringSubmatch(text); m != nil {
			total, _ := strconv.Atoi(m[1])
			if total > 0 {
				d.GpusPerNode = 8
				d.Nodes = (total + 7) / 8
				if total < 8 {
					d.GpusPerNode = total
					d.Nodes = 1
				}
			}
		}
	}
	if d.Nodes > 0 && d.GpusPerNode == 0 {
		d.GpusPerNode = 8
	}

	if m := reMonthsZh.FindStringSubmatch(text); m != nil {
		d.TermMonths, _ = strconv.Atoi(m[1])
	} else if m := reMonthsEn.FindStringSubmatch(text); m != nil {
		d.TermMonths, _ = strconv.Atoi(m[1])
	} else if m := reYearsZh.FindStringSubmatch(text); m != nil {
		d.TermMonths = 12 * cnNumber(m[1])
	} else if m := reYearsEn.FindStringSubmatch(text); m != nil {
		d.TermMonths = 12 * cnNumber(m[1])
	} else if strings.Contains(lower, "半年") {
		d.TermMonths = 6
	}

	now := time.Now()
	if m := reISODate.FindStringSubmatch(text); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		da, _ := strconv.Atoi(m[3])
		d.StartDate = fmt.Sprintf("%04d-%02d-%02d", y, mo, da)
	} else if m := reDateZh.FindStringSubmatch(text); m != nil {
		mo, _ := strconv.Atoi(m[1])
		da, _ := strconv.Atoi(m[2])
		d.StartDate = nextDate(now, mo, da)
	} else if m := reDateEn.FindStringSubmatch(text); m != nil {
		mo := monthIndex(m[1])
		da, _ := strconv.Atoi(m[2])
		d.StartDate = nextDate(now, mo, da)
	}

	if m := rePriceRange.FindStringSubmatch(text); m != nil {
		lo, _ := strconv.ParseFloat(m[1], 64)
		hi, _ := strconv.ParseFloat(m[2], 64)
		if lo > 0 && hi >= lo && hi < 1000 {
			d.TargetPriceMin, d.TargetPriceMax, d.PriceCeiling = lo, hi, hi
		}
	}
	if d.PriceCeiling == 0 {
		if m := rePriceOne.FindStringSubmatch(text); m != nil {
			for _, g := range m[1:] {
				if g != "" {
					v, _ := strconv.ParseFloat(g, 64)
					if v > 0 && v < 1000 {
						d.PriceCeiling = v
					}
				}
			}
		}
	}

	for _, r := range regionKeywords {
		for _, k := range r.keys {
			if strings.Contains(lower, k) {
				d.Regions = append(d.Regions, r.code)
				break
			}
		}
	}

	switch {
	case strings.Contains(lower, "裸金属") || strings.Contains(lower, "bare metal") || strings.Contains(lower, "bare-metal"):
		d.Delivery = "bare_metal"
	case strings.Contains(lower, "slurm"):
		d.Delivery = "slurm"
	case strings.Contains(lower, "k8s") || strings.Contains(lower, "kubernetes"):
		d.Delivery = "k8s"
	case strings.Contains(lower, "虚拟机") || regexp.MustCompile(`\bvms?\b`).MatchString(lower):
		d.Delivery = "vm"
	}
	if m := reIB.FindStringSubmatch(text); m != nil {
		d.Interconnect = "IB " + m[1] + "G"
		if strings.Contains(strings.ToLower(m[0]), "roce") {
			d.Interconnect = "RoCE " + m[1] + "G"
		}
	} else if strings.Contains(lower, "infiniband") || regexp.MustCompile(`\bib\b`).MatchString(lower) {
		d.Interconnect = "IB"
	}
	if m := reStorage.FindStringSubmatch(text); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		if strings.EqualFold(m[2], "PB") {
			v *= 1000
		}
		d.StorageTB = v
	}
	for _, c := range []struct{ key, label string }{
		{"soc2", "SOC 2"}, {"soc 2", "SOC 2"}, {"iso27001", "ISO 27001"}, {"iso 27001", "ISO 27001"}, {"iso", "ISO 27001"},
		{"hipaa", "HIPAA"}, {"gdpr", "GDPR"}, {"数据不出境", "Data residency"}, {"不能出", "Data residency"}, {"data residency", "Data residency"},
	} {
		if strings.Contains(lower, c.key) && !containsString(d.Compliance, c.label) {
			d.Compliance = append(d.Compliance, c.label)
		}
	}
	if m := reSLA.FindStringSubmatch(text); m != nil {
		d.Compliance = append(d.Compliance, "SLA "+m[1]+"%")
	}
	if m := reTier.FindStringSubmatch(text); m != nil {
		t := strings.ToUpper(m[1])
		if t == "III" {
			t = "3"
		} else if t == "IV" {
			t = "4"
		}
		d.Compliance = append(d.Compliance, "Tier "+t+"+")
	}
	d.Notes = text
	return d
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func cnNumber(s string) int {
	switch strings.ToLower(s) {
	case "一", "1", "a", "one":
		return 1
	case "二", "两", "2", "two":
		return 2
	case "三", "3", "three":
		return 3
	}
	n, _ := strconv.Atoi(s)
	return n
}

func monthIndex(m string) int {
	m = strings.ToLower(m)
	if len(m) > 3 {
		m = m[:3]
	}
	months := []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	for i, v := range months {
		if v == m {
			return i + 1
		}
	}
	return 0
}

// nextDate returns the next occurrence of month/day at or after today.
func nextDate(now time.Time, month, day int) string {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return ""
	}
	y := now.Year()
	candidate := time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if candidate.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) {
		candidate = candidate.AddDate(1, 0, 0)
	}
	return candidate.Format("2006-01-02")
}

// ---- optional LLM path ----

const rfqParseSystemPrompt = `You extract GPU compute rental requests into JSON. Return ONLY a JSON object with these optional keys:
gpu_model (e.g. "B300","B200","H200","H100","A100","RTX 4090","M3 Ultra"), gpus_per_node (int), nodes (int), term_months (int),
start_date ("YYYY-MM-DD"), price_ceiling (number, USD per GPU-hour, the maximum the buyer will pay),
target_price_min, target_price_max (numbers, USD per GPU-hour), regions (array of codes from JP,SG,KR,US-WEST,US-EAST,EU,HK,TW,IN,AU,ME),
delivery ("bare_metal"|"vm"|"slurm"|"k8s"), interconnect (e.g. "IB 400G"), storage_tb (number), compliance (array of strings like "SOC 2","ISO 27001","Tier 3+","SLA 99.5%","Data residency").
Omit keys you cannot infer. Never invent numbers.`

func parseRFQWithAI(ctx context.Context, text string) (*RFQDraft, error) {
	apiKey := strings.TrimSpace(operation_setting.GetMonitorAIAnalysisAPIKey())
	if apiKey == "" {
		return nil, fmt.Errorf("ai parse not configured")
	}
	base := strings.TrimRight(strings.TrimSpace(operation_setting.GetMonitorAIAnalysisBaseURL()), "/")
	if base == "" {
		return nil, fmt.Errorf("ai parse base url empty")
	}
	if !strings.HasSuffix(base, "/chat/completions") {
		base = strings.TrimSuffix(base, "/responses") + "/chat/completions"
	}
	modelName := strings.TrimSpace(operation_setting.GetMonitorAIAnalysisModel())
	if modelName == "" {
		modelName = "gpt-5-mini"
	}
	if len(text) > 6000 {
		text = text[:6000]
	}
	payload := map[string]any{
		"model":  modelName,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": rfqParseSystemPrompt},
			{"role": "user", "content": text},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("ai parse: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("ai parse: empty response")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	content = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(content, "```json"), "```"), "```")
	var d RFQDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &d); err != nil {
		return nil, err
	}
	// Merge heuristic values for anything the model left blank.
	h := ParseRFQHeuristic(text)
	if d.GpuModel == "" {
		d.GpuModel = h.GpuModel
	}
	if d.Nodes == 0 {
		d.Nodes = h.Nodes
	}
	if d.GpusPerNode == 0 {
		d.GpusPerNode = h.GpusPerNode
	}
	if d.TermMonths == 0 {
		d.TermMonths = h.TermMonths
	}
	if d.StartDate == "" {
		d.StartDate = h.StartDate
	}
	if d.PriceCeiling == 0 {
		d.PriceCeiling = h.PriceCeiling
	}
	if d.TargetPriceMax == 0 {
		d.TargetPriceMin, d.TargetPriceMax = h.TargetPriceMin, h.TargetPriceMax
	}
	if len(d.Regions) == 0 {
		d.Regions = h.Regions
	}
	if d.Delivery == "" {
		d.Delivery = h.Delivery
	}
	if d.Interconnect == "" {
		d.Interconnect = h.Interconnect
	}
	if d.StorageTB == 0 {
		d.StorageTB = h.StorageTB
	}
	if len(d.Compliance) == 0 {
		d.Compliance = h.Compliance
	}
	d.Notes = text
	return &d, nil
}

package service

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"github.com/QuantumNous/new-api/common"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// The catalogue ships inside the binary. Production runs several nodes behind a
// load balancer; a spec directory mounted per node is one more thing that can
// be missing on exactly one of them, and a node that boots with an empty
// catalogue answers "unknown tool" for endpoints its neighbours serve fine.
//
//go:embed toolspecs/*.json
var embeddedToolSpecs embed.FS

// The tool registry is the "more tools" half of flatkey's value proposition:
// one key that buys models and data endpoints alike.
//
// A tool is a declarative JSON spec on disk. Adding a tool means adding a file,
// and promoting a resold tool to a direct upstream contract means editing that
// file's adapter block — no code change either way.
//
// Multi-node note (Rule 11): the registry is read-only after load. Every node
// loads the same spec files independently and never mutates them at runtime, so
// there is no cross-node state to coordinate. Everything that must be
// consistent across nodes — quota, usage rows — goes through the database.

type ToolMode string

const (
	// ToolModeNative is a direct upstream contract. Full margin.
	ToolModeNative ToolMode = "native"
	// ToolModeFederated is resold through an aggregator. Thin margin.
	ToolModeFederated ToolMode = "federated"
)

type ToolPricingModel string

const (
	ToolPerCall   ToolPricingModel = "per_call"
	ToolPerResult ToolPricingModel = "per_result"
	ToolFree      ToolPricingModel = "free"
)

// ToolMinMarginRate is the floor on gross margin for every billed tool call.
// A spec's declared price is never trusted below this: whatever the catalogue
// says, we charge at least cost / (1 - rate). Enforcing it here rather than in
// the specs means a bad generated price can never sell at a loss.
const ToolMinMarginRate = 0.20

type ToolPricing struct {
	Model ToolPricingModel `json:"model"`
	// Amount is USD charged to the caller, per call or per returned item.
	Amount float64 `json:"amount"`
	// Base is a fixed USD fee added on top of a per_result charge.
	Base float64 `json:"base,omitempty"`
	// Cost is what we pay upstream. Drives the margin floor and the realised
	// margin shown in the console; never returned to callers.
	Cost float64 `json:"cost,omitempty"`
	// PayOnMatch suppresses the charge when a call returns nothing.
	PayOnMatch bool `json:"payOnMatch,omitempty"`
}

type ToolField struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
	Example     any      `json:"example,omitempty"`
}

type ToolInputSchema struct {
	Type       string               `json:"type"`
	Properties map[string]ToolField `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

type ToolAuth struct {
	// Type is one of: none, header, bearer, query.
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	// EnvKey names the environment variable holding the credential. The secret
	// itself never appears in a spec file.
	EnvKey string `json:"envKey,omitempty"`
}

type ToolAdapter struct {
	Kind string `json:"kind"` // http | mcp | monid | deepline | waterfall

	Method     string            `json:"method,omitempty"`
	URL        string            `json:"url,omitempty"`
	Auth       *ToolAuth         `json:"auth,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
	Body       map[string]any    `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	ResultPath string            `json:"resultPath,omitempty"`

	ToolName string   `json:"toolName,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Endpoint string   `json:"endpoint,omitempty"`
	Tool     string   `json:"tool,omitempty"`
	Steps    []string `json:"steps,omitempty"`
}

type ToolSpec struct {
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Provider    string          `json:"provider"`
	Mode        ToolMode        `json:"mode"`
	Categories  []string        `json:"categories"`
	Description string          `json:"description"`
	Keywords    []string        `json:"keywords,omitempty"`
	Input       ToolInputSchema `json:"input"`
	Pricing     ToolPricing     `json:"pricing"`
	Adapter     ToolAdapter     `json:"adapter"`
	Enabled     *bool           `json:"enabled,omitempty"`
	DocsURL     string          `json:"docsUrl,omitempty"`
	Settlement  string          `json:"settlement,omitempty"`
}

func (s *ToolSpec) IsEnabled() bool { return s.Enabled == nil || *s.Enabled }

// Platform is the second path segment of the provider ("tikhub:douyin" ->
// "douyin"), used for the marketplace's platform filter chips.
func (s *ToolSpec) Platform() string {
	if i := strings.Index(s.Provider, ":"); i >= 0 && i+1 < len(s.Provider) {
		return s.Provider[i+1:]
	}
	return s.Provider
}

// PriceUSD returns what to charge for a call that produced resultCount items,
// with the margin floor applied. A failed call never reaches here; a call that
// found nothing is free when the tool is pay-on-match.
func (s *ToolSpec) PriceUSD(resultCount int) float64 {
	p := s.Pricing
	if p.Model == ToolFree {
		return 0
	}
	if p.PayOnMatch && resultCount == 0 {
		return 0
	}

	unit := marginFloored(p.Amount, p.Cost)
	if p.Model == ToolPerResult {
		return marginFloored(p.Base, 0) + unit*float64(resultCount)
	}
	return unit
}

// marginFloored keeps every billed unit at or above the minimum gross margin.
func marginFloored(price, cost float64) float64 {
	if cost <= 0 {
		return price
	}
	floor := cost / (1 - ToolMinMarginRate)
	if price < floor {
		return floor
	}
	return price
}

// MaxPriceUSD is the worst case for one call, used to refuse before dispatch.
func (s *ToolSpec) MaxPriceUSD() float64 {
	if s.Pricing.Model == ToolFree {
		return 0
	}
	unit := marginFloored(s.Pricing.Amount, s.Pricing.Cost)
	if s.Pricing.Model == ToolPerResult {
		return marginFloored(s.Pricing.Base, 0) + unit
	}
	return unit
}

// ToolQuarantine records tools observed to fail for reasons that will recur —
// a missing upstream credential, an exhausted quota, an unprovisioned account.
// Upstream catalogue metadata is not trustworthy on this point, so availability
// is decided by what we measured.
type ToolQuarantine struct {
	Tools     map[string]toolQuarantineEntry `json:"tools"`
	Providers map[string]toolQuarantineEntry `json:"providers"`
}

type toolQuarantineEntry struct {
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type ToolRegistry struct {
	specs      []*ToolSpec
	byId       map[string]*ToolSpec
	quarantine ToolQuarantine
	docFreq    map[string]int
}

var (
	toolRegistry     *ToolRegistry
	toolRegistryOnce sync.Once
	toolRegistryErr  error
)

// ToolSpecDir is an optional override for iterating on specs without a
// rebuild. Empty means the embedded catalogue is used, which is the normal
// case including in production.
func ToolSpecDir() string {
	return os.Getenv("TOOL_SPEC_DIR")
}

// GetToolRegistry loads the catalogue once per process.
func GetToolRegistry() (*ToolRegistry, error) {
	toolRegistryOnce.Do(func() {
		toolRegistry, toolRegistryErr = loadToolRegistry(ToolSpecDir())
		if toolRegistryErr != nil {
			common.SysError("tool registry unavailable: " + toolRegistryErr.Error())
			// An empty registry keeps every caller working; the marketplace
			// just shows nothing rather than 500-ing.
			toolRegistry = emptyToolRegistry()
		} else {
			common.SysLog(fmt.Sprintf("tool registry loaded: %d tools across %d providers",
				toolRegistry.Len(), len(toolRegistry.Providers())))
		}
	})
	return toolRegistry, nil
}

func emptyToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		byId:    map[string]*ToolSpec{},
		docFreq: map[string]int{},
		quarantine: ToolQuarantine{
			Tools:     map[string]toolQuarantineEntry{},
			Providers: map[string]toolQuarantineEntry{},
		},
	}
}

// toolSpecSource abstracts over the embedded catalogue and an on-disk override
// so the parsing below has one code path and cannot drift between them.
type toolSpecSource struct {
	label string
	names []string
	read  func(name string) ([]byte, error)
}

func embeddedToolSpecSource() (*toolSpecSource, error) {
	entries, err := fs.ReadDir(embeddedToolSpecs, "toolspecs")
	if err != nil {
		return nil, fmt.Errorf("read embedded tool specs: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return &toolSpecSource{
		label: "embedded",
		names: names,
		read: func(name string) ([]byte, error) {
			return embeddedToolSpecs.ReadFile(filepath.ToSlash(filepath.Join("toolspecs", name)))
		},
	}, nil
}

func dirToolSpecSource(dir string) (*toolSpecSource, error) {
	specDir := filepath.Join(dir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, fmt.Errorf("read tool spec dir %s: %w", specDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return &toolSpecSource{
		label: specDir,
		names: names,
		read:  func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(specDir, name)) },
	}, nil
}

func loadToolRegistry(dir string) (*ToolRegistry, error) {
	var (
		src *toolSpecSource
		err error
	)
	if dir != "" {
		src, err = dirToolSpecSource(dir)
	} else {
		src, err = embeddedToolSpecSource()
	}
	if err != nil {
		return nil, err
	}

	reg := emptyToolRegistry()
	for _, name := range src.names {
		raw, err := src.read(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		// A spec file may hold one tool or an array, so a bulk-imported
		// provider catalogue fits in a single file.
		var batch []*ToolSpec
		if err := common.Unmarshal(raw, &batch); err != nil {
			var one ToolSpec
			if err2 := common.Unmarshal(raw, &one); err2 != nil {
				return nil, fmt.Errorf("%s is not a tool spec or spec array: %w", name, err)
			}
			batch = []*ToolSpec{&one}
		}
		for _, s := range batch {
			if s == nil || !s.IsEnabled() || s.Id == "" {
				continue
			}
			if _, dup := reg.byId[s.Id]; dup {
				return nil, fmt.Errorf("duplicate tool id %q in %s", s.Id, name)
			}
			reg.byId[s.Id] = s
			reg.specs = append(reg.specs, s)
		}
	}

	sort.Slice(reg.specs, func(i, j int) bool { return reg.specs[i].Id < reg.specs[j].Id })

	// Quarantine is optional — a fresh checkout has measured nothing yet. It is
	// only read from an override directory: it records what our own calls
	// measured, so it is operational state, not something to bake into a build.
	if raw, err := readToolQuarantine(dir); err == nil {
		var q ToolQuarantine
		if err := common.Unmarshal(raw, &q); err == nil {
			if q.Tools != nil {
				reg.quarantine.Tools = q.Tools
			}
			if q.Providers != nil {
				reg.quarantine.Providers = q.Providers
			}
		}
	}

	reg.buildIndex()
	common.SysLog(fmt.Sprintf("tool catalogue source: %s (%d files)", src.label, len(src.names)))
	return reg, nil
}

func readToolQuarantine(dir string) ([]byte, error) {
	if dir == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(dir, "quarantine.json"))
}

func (r *ToolRegistry) buildIndex() {
	r.docFreq = make(map[string]int, 4096)
	for _, s := range r.specs {
		for t := range toolTokenSet(s) {
			r.docFreq[t]++
		}
	}
}

func (r *ToolRegistry) Len() int { return len(r.specs) }

func (r *ToolRegistry) Get(id string) (*ToolSpec, bool) {
	s, ok := r.byId[id]
	return s, ok
}

func (r *ToolRegistry) All() []*ToolSpec { return r.specs }

// QuarantineReason returns why a tool is held back, or "" when it is healthy.
func (r *ToolRegistry) QuarantineReason(s *ToolSpec) string {
	if e, ok := r.quarantine.Tools[s.Id]; ok {
		return e.Reason
	}
	if e, ok := r.quarantine.Providers[s.Provider]; ok {
		return e.Reason
	}
	return ""
}

// ToolProviderStat summarises one upstream for the marketplace filters.
type ToolProviderStat struct {
	Provider string `json:"provider"`
	Platform string `json:"platform"`
	Tools    int    `json:"tools"`
	Native   int    `json:"native"`
}

func (r *ToolRegistry) Providers() []ToolProviderStat {
	agg := map[string]*ToolProviderStat{}
	for _, s := range r.specs {
		st := agg[s.Provider]
		if st == nil {
			st = &ToolProviderStat{Provider: s.Provider, Platform: s.Platform()}
			agg[s.Provider] = st
		}
		st.Tools++
		if s.Mode == ToolModeNative {
			st.Native++
		}
	}
	out := make([]ToolProviderStat, 0, len(agg))
	for _, v := range agg {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tools != out[j].Tools {
			return out[i].Tools > out[j].Tools
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

// Platforms powers the marketplace's platform chips (Douyin, Xiaohongshu, ...).
type ToolPlatformStat struct {
	Platform string `json:"platform"`
	Tools    int    `json:"tools"`
}

func (r *ToolRegistry) Platforms() []ToolPlatformStat {
	agg := map[string]int{}
	for _, s := range r.specs {
		agg[s.Platform()]++
	}
	out := make([]ToolPlatformStat, 0, len(agg))
	for p, n := range agg {
		out = append(out, ToolPlatformStat{Platform: p, Tools: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tools != out[j].Tools {
			return out[i].Tools > out[j].Tools
		}
		return out[i].Platform < out[j].Platform
	})
	return out
}

// Categories powers the marketplace's category chips.
func (r *ToolRegistry) Categories() []ToolPlatformStat {
	agg := map[string]int{}
	for _, s := range r.specs {
		for _, c := range s.Categories {
			agg[c]++
		}
	}
	out := make([]ToolPlatformStat, 0, len(agg))
	for c, n := range agg {
		out = append(out, ToolPlatformStat{Platform: c, Tools: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tools != out[j].Tools {
			return out[i].Tools > out[j].Tools
		}
		return out[i].Platform < out[j].Platform
	})
	return out
}

// ToolHit is one discover result.
type ToolHit struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Provider    string      `json:"provider"`
	Platform    string      `json:"platform"`
	Mode        ToolMode    `json:"mode"`
	Description string      `json:"description"`
	Categories  []string    `json:"categories"`
	Pricing     ToolPricing `json:"pricing"`
	Score       float64     `json:"score"`
}

const maxToolDiscoverLimit = 40

// Discover is the search primitive an agent calls before it knows which tool it
// wants. Exposing a search instead of thousands of tool definitions is what
// lets the catalogue grow without overrunning an agent's context window.
func (r *ToolRegistry) Discover(query string, limit int, minScore float64) []ToolHit {
	terms := toolTokenize(query)
	if len(terms) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > maxToolDiscoverLimit {
		limit = maxToolDiscoverLimit
	}
	if minScore <= 0 {
		minScore = 0.2
	}

	n := float64(len(r.specs))
	hits := make([]ToolHit, 0, limit*4)

	for _, s := range r.specs {
		// A tool measured as broken is never handed to an agent.
		if r.QuarantineReason(s) != "" {
			continue
		}
		toks := toolTokenSet(s)
		nameToks := toolTokenize(s.Name)

		var score float64
		for _, term := range terms {
			if !toolHasTerm(toks, term) {
				continue
			}
			idf := math.Log(1 + n/(1+float64(r.docFreq[term])))
			weight := 1.0
			if containsToolStr(nameToks, term) || strings.Contains(s.Id, term) {
				weight = 2.5
			}
			score += idf * weight
		}
		if score <= 0 {
			continue
		}
		// Normalise into (0,1), then nudge direct-contract tools up: when two
		// tools answer the same question we would rather run the one with the
		// better margin and the better measured reliability.
		norm := score / (score + float64(len(terms))*1.6)
		if s.Mode == ToolModeNative {
			norm *= 1.12
		}
		if norm > 0.999 {
			norm = 0.999
		}
		if norm < minScore {
			continue
		}
		hits = append(hits, ToolHit{
			Id: s.Id, Name: s.Name, Provider: s.Provider, Platform: s.Platform(),
			Mode: s.Mode, Description: s.Description, Categories: s.Categories,
			Pricing: s.Pricing, Score: math.Round(norm*10000) / 10000,
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Id < hits[j].Id
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

var toolStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "for": true, "to": true,
	"and": true, "or": true, "in": true, "on": true, "by": true, "with": true,
	"get": true, "from": true,
}

func toolTokenize(s string) []string {
	if s == "" {
		return nil
	}
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return false
		}
		// Keep CJK so Chinese queries tokenise into runs rather than vanishing.
		if r >= 0x4E00 && r <= 0x9FFF {
			return false
		}
		return true
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) < 2 || toolStopWords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

func toolTokenSet(s *ToolSpec) map[string]struct{} {
	set := make(map[string]struct{}, 48)
	add := func(str string) {
		for _, t := range toolTokenize(str) {
			set[t] = struct{}{}
		}
	}
	add(s.Name)
	add(s.Description)
	add(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(s.Id))
	add(s.Provider)
	for _, c := range s.Categories {
		add(c)
	}
	for _, k := range s.Keywords {
		add(k)
	}
	return set
}

// toolHasTerm allows a cheap prefix match so "review" finds "reviews".
func toolHasTerm(set map[string]struct{}, term string) bool {
	if _, ok := set[term]; ok {
		return true
	}
	for t := range set {
		if strings.HasPrefix(t, term) || strings.HasPrefix(term, t) {
			return true
		}
	}
	return false
}

func containsToolStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

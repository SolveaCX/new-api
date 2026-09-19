package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Compute demand leads — the zero-friction intake behind "post your GPU need
// in 3 seconds" on the marketing site. No account required: a contact, a GPU
// model, a GPU count and a term are enough. Leads roll up into per-GPU demand
// pools so many small requests read as one large order to suppliers, and each
// lead gets a share code so it can bring peers into the same pool.

const (
	ComputeDemandLeadStatusNew       = "new"
	ComputeDemandLeadStatusContacted = "contacted"
	ComputeDemandLeadStatusConverted = "converted"
	computeDemandLeadCodeLength      = 8
)

type ComputeDemandLead struct {
	Id           int     `json:"id"`
	Code         string  `json:"code" gorm:"type:varchar(16);uniqueIndex"`
	Contact      string  `json:"-" gorm:"type:varchar(191)"`
	Company      string  `json:"-" gorm:"type:varchar(191)"`
	GpuModel     string  `json:"gpu_model" gorm:"column:gpu_model;type:varchar(64);index"`
	Gpus         int     `json:"gpus"`
	TermMonths   int     `json:"term_months" gorm:"column:term_months"`
	Regions      string  `json:"regions" gorm:"type:varchar(191)"`
	StartDate    string  `json:"start_date" gorm:"column:start_date;type:varchar(16)"`
	PriceCeiling float64 `json:"price_ceiling" gorm:"column:price_ceiling"`
	Notes        string  `json:"-" gorm:"type:text"`
	Source       string  `json:"source" gorm:"type:varchar(64)"`
	ReferredBy   string  `json:"referred_by" gorm:"column:referred_by;type:varchar(16);index"`
	ClientIP     string  `json:"-" gorm:"column:client_ip;type:varchar(64)"`
	Status       string  `json:"status" gorm:"type:varchar(32);index;default:new"`
	CreatedTime  int64   `json:"created_time" gorm:"column:created_time;bigint"`
}

func (ComputeDemandLead) TableName() string { return "compute_demand_leads" }

// ComputeDemandLeadAdminView adds the private fields for ops.
type ComputeDemandLeadAdminView struct {
	ComputeDemandLead
	Contact   string `json:"contact"`
	Company   string `json:"company"`
	Notes     string `json:"notes"`
	Referrals int64  `json:"referrals"`
}

// Insert stores a lead, generating a unique share code.
func (lead *ComputeDemandLead) Insert() error {
	lead.CreatedTime = common.GetTimestamp()
	if lead.Status == "" {
		lead.Status = ComputeDemandLeadStatusNew
	}
	for attempt := 0; attempt < 5; attempt++ {
		lead.Code = strings.ToUpper(common.GetRandomString(computeDemandLeadCodeLength))
		err := DB.Create(lead).Error
		if err == nil {
			return nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return err
		}
	}
	return errors.New("could not allocate a share code")
}

func GetComputeDemandLeadByCode(code string) (*ComputeDemandLead, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var lead ComputeDemandLead
	if err := DB.First(&lead, "code = ?", code).Error; err != nil {
		return nil, err
	}
	return &lead, nil
}

// CountComputeDemandReferrals counts leads that joined through a share code.
func CountComputeDemandReferrals(code string) (int64, error) {
	var n int64
	err := DB.Model(&ComputeDemandLead{}).Where("referred_by = ?", strings.ToUpper(strings.TrimSpace(code))).Count(&n).Error
	return n, err
}

// ListComputeDemandLeads returns the newest leads with private fields (admin).
func ListComputeDemandLeads(limit int) ([]*ComputeDemandLeadAdminView, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var leads []*ComputeDemandLead
	if err := DB.Order("id desc").Limit(limit).Find(&leads).Error; err != nil {
		return nil, err
	}
	out := make([]*ComputeDemandLeadAdminView, 0, len(leads))
	for _, l := range leads {
		refs, _ := CountComputeDemandReferrals(l.Code)
		out = append(out, &ComputeDemandLeadAdminView{ComputeDemandLead: *l, Contact: l.Contact, Company: l.Company, Notes: l.Notes, Referrals: refs})
	}
	return out, nil
}

// ComputeDemandPool is the public roll-up of demand for one GPU model.
type ComputeDemandPool struct {
	GpuModel    string  `json:"gpu_model"`
	Gpus        int     `json:"gpus"`
	Requests    int     `json:"requests"`
	Leads       int     `json:"leads"`
	Tier        int     `json:"tier"`
	NextTierGpu int     `json:"next_tier_gpus"`
	Discount    float64 `json:"discount_pct"`
}

// Pool tiers: total GPUs pooled → indicative discount vs. on-demand list.
var computeDemandPoolTiers = []struct {
	Gpus     int
	Discount float64
}{{64, 5}, {256, 10}, {1024, 18}, {4096, 25}}

func computeDemandPoolTier(gpus int) (tier int, next int, discount float64) {
	for i, t := range computeDemandPoolTiers {
		if gpus >= t.Gpus {
			tier, discount = i+1, t.Discount
		} else {
			return tier, t.Gpus, discount
		}
	}
	return tier, 0, discount
}

// GetComputeDemandPools aggregates open RFQs and new leads per GPU model.
func GetComputeDemandPools() ([]*ComputeDemandPool, error) {
	pools := map[string]*ComputeDemandPool{}
	get := func(g string) *ComputeDemandPool {
		g = strings.TrimSpace(g)
		if pools[g] == nil {
			pools[g] = &ComputeDemandPool{GpuModel: g}
		}
		return pools[g]
	}
	var rfqs []struct {
		GpuModel string
		Gpus     int
		N        int
	}
	if err := DB.Model(&ComputeRFQ{}).Select("gpu_model, sum(gpus_per_node*nodes) as gpus, count(*) as n").
		Where("status in ?", []string{ComputeRFQStatusMatching, ComputeRFQStatusChoosing}).Group("gpu_model").Scan(&rfqs).Error; err != nil {
		return nil, err
	}
	for _, r := range rfqs {
		p := get(r.GpuModel)
		p.Gpus += r.Gpus
		p.Requests += r.N
	}
	var leads []struct {
		GpuModel string
		Gpus     int
		N        int
	}
	if err := DB.Model(&ComputeDemandLead{}).Select("gpu_model, sum(gpus) as gpus, count(*) as n").
		Where("status <> ?", ComputeDemandLeadStatusConverted).Group("gpu_model").Scan(&leads).Error; err != nil {
		return nil, err
	}
	for _, l := range leads {
		p := get(l.GpuModel)
		p.Gpus += l.Gpus
		p.Leads += l.N
	}
	out := make([]*ComputeDemandPool, 0, len(pools))
	for _, p := range pools {
		p.Tier, p.NextTierGpu, p.Discount = computeDemandPoolTier(p.Gpus)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Gpus > out[j].Gpus })
	return out, nil
}

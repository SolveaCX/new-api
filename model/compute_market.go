package model

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Compute Market — buyer RFQs, supplier bids and supplier profiles.
//
// Mechanism (product decision 2026-09-16): a buyer posts a structured RFQ, it
// enters "matching" for 24h; verified suppliers bid publicly (reverse auction,
// bids only go down, last-15-minute bids extend the window); the buyer may
// accept any eligible bid at any time while matching. If nothing is accepted
// by the deadline the RFQ moves to "choosing" and the buyer picks from the
// three lowest eligible bids (or extends matching).
//
// ANONYMITY: buyer and supplier user ids are never serialized. Each side is
// shown to the other only as a stable alias (#B-1234 / #S-5678). Contact
// details never travel through these models.

const (
	ComputeRFQStatusMatching   = "matching"
	ComputeRFQStatusChoosing   = "choosing"
	ComputeRFQStatusMatched    = "matched"
	ComputeRFQStatusContracted = "contracted"
	ComputeRFQStatusLive       = "live"
	ComputeRFQStatusCompleted  = "completed"
	ComputeRFQStatusCancelled  = "cancelled"

	ComputeBidStatusLive      = "live"
	ComputeBidStatusAccepted  = "accepted"
	ComputeBidStatusRejected  = "rejected"
	ComputeBidStatusWithdrawn = "withdrawn"

	ComputeMarketMatchingWindowSeconds = 24 * 3600
	ComputeMarketAntiSnipeSeconds      = 15 * 60
	ComputeMarketMaxExtensions         = 3
	ComputeMarketTopN                  = 3
	ComputeMarketMinDecrement          = 0.05
	ComputeSupplierLevelRegistered     = 0
	ComputeSupplierLevelVerified       = 1
	ComputeSupplierLevelPreferred      = 2
)

var (
	ErrComputeRFQNotMatching   = errors.New("this request is no longer accepting bids")
	ErrComputeBidTooHigh       = errors.New("bid must be below the price ceiling")
	ErrComputeBidNotLower      = errors.New("a new bid must be at least $0.05 lower than your current bid")
	ErrComputeBidNotAcceptable = errors.New("this bid can no longer be accepted")
	ErrComputeRFQNotOwner      = errors.New("request not found")
	ErrComputeRFQMaxExtensions = errors.New("this request has already been extended the maximum number of times")
	ErrComputeCeilingNotHigher = errors.New("the new ceiling must be higher than the current one")
)

// ComputeRFQ is a buyer's structured compute request.
type ComputeRFQ struct {
	Id     int `json:"id"`
	UserId int `json:"-" gorm:"column:user_id;index"`

	GpuModel     string  `json:"gpu_model" gorm:"column:gpu_model;type:varchar(64);index"`
	GpusPerNode  int     `json:"gpus_per_node" gorm:"column:gpus_per_node;default:8"`
	Nodes        int     `json:"nodes" gorm:"default:1"`
	TermMonths   int     `json:"term_months" gorm:"column:term_months;default:1"`
	StartDate    string  `json:"start_date" gorm:"column:start_date;type:varchar(16)"`
	PriceCeiling float64 `json:"price_ceiling" gorm:"column:price_ceiling"`
	// Target price band is private to the buyer (and flatkey ops) — never
	// shown to suppliers.
	TargetPriceMin float64 `json:"-" gorm:"column:target_price_min"`
	TargetPriceMax float64 `json:"-" gorm:"column:target_price_max"`

	Regions      string  `json:"regions" gorm:"type:varchar(191)"`
	Delivery     string  `json:"delivery" gorm:"type:varchar(64)"`
	Interconnect string  `json:"interconnect" gorm:"type:varchar(64)"`
	StorageTB    float64 `json:"storage_tb" gorm:"column:storage_tb"`
	Compliance   string  `json:"compliance" gorm:"type:varchar(191)"`
	PaymentTerms string  `json:"payment_terms" gorm:"column:payment_terms;type:varchar(191)"`
	Notes        string  `json:"notes" gorm:"type:text"`

	Status           string `json:"status" gorm:"type:varchar(32);index;default:matching"`
	MatchingDeadline int64  `json:"matching_deadline" gorm:"column:matching_deadline;bigint"`
	Extensions       int    `json:"extensions" gorm:"default:0"`
	AcceptedBidId    int    `json:"accepted_bid_id" gorm:"column:accepted_bid_id;default:0"`
	// Featured requests are pinned to the top of the marketplace (ops-curated).
	Featured    bool  `json:"featured" gorm:"column:featured;index"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;bigint"`
	UpdatedTime int64 `json:"updated_time" gorm:"column:updated_time;bigint"`

	// Derived, never stored.
	Code       string `json:"code" gorm:"-"`
	BuyerAlias string `json:"buyer_alias" gorm:"-"`
	Mine       bool   `json:"mine" gorm:"-"`
}

func (ComputeRFQ) TableName() string { return "compute_rfqs" }

// ComputeBid is one supplier's current price on an RFQ (one live bid per
// supplier per RFQ; re-bidding lowers the same row).
type ComputeBid struct {
	Id             int `json:"id"`
	RfqId          int `json:"rfq_id" gorm:"column:rfq_id;index"`
	SupplierUserId int `json:"-" gorm:"column:supplier_user_id;index"`

	PricePerGpuHour float64 `json:"price_per_gpu_hour" gorm:"column:price_per_gpu_hour"`
	DeliverDate     string  `json:"deliver_date" gorm:"column:deliver_date;type:varchar(16)"`
	SlaPct          float64 `json:"sla_pct" gorm:"column:sla_pct"`
	// HardReqs is a JSON object {requirement: bool} declared by the supplier;
	// any false makes the bid visible but ineligible (cannot be accepted).
	HardReqs    string `json:"hard_reqs" gorm:"column:hard_reqs;type:text"`
	Deviations  string `json:"deviations" gorm:"type:varchar(512)"`
	Eligible    bool   `json:"eligible"`
	Status      string `json:"status" gorm:"type:varchar(32);index;default:live"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;bigint"`

	SupplierAlias string `json:"supplier_alias" gorm:"-"`
	SupplierLevel int    `json:"supplier_level" gorm:"-"`
	Mine          bool   `json:"mine" gorm:"-"`
	Rank          int    `json:"rank" gorm:"-"`
}

func (ComputeBid) TableName() string { return "compute_bids" }

// ComputeSupplier is a supplier profile. Level 0 = registered (can browse),
// 1 = verified (can bid), 2 = preferred. Company/contact are only returned to
// the owner and admins.
type ComputeSupplier struct {
	Id                 int     `json:"id"`
	UserId             int     `json:"user_id" gorm:"column:user_id;uniqueIndex"`
	Company            string  `json:"company" gorm:"type:varchar(191)"`
	Contact            string  `json:"contact" gorm:"type:varchar(191)"`
	Regions            string  `json:"regions" gorm:"type:varchar(191)"`
	Inventory          string  `json:"inventory" gorm:"type:text"`
	Level              int     `json:"level" gorm:"default:0;index"`
	Rating             float64 `json:"rating" gorm:"default:0"`
	CompletedContracts int     `json:"completed_contracts" gorm:"column:completed_contracts;default:0"`
	CreatedTime        int64   `json:"created_time" gorm:"column:created_time;bigint"`
	UpdatedTime        int64   `json:"updated_time" gorm:"column:updated_time;bigint"`
}

func (ComputeSupplier) TableName() string { return "compute_suppliers" }

// ComputeMarketAlias returns the stable anonymous alias for a user on one side
// of the market, e.g. "#B-4821" or "#S-1937". Not reversible by clients.
func ComputeMarketAlias(prefix string, userId int) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("flatkey-compute-market:%s:%d", prefix, userId)))
	return fmt.Sprintf("#%s-%04d", prefix, 1000+h.Sum32()%9000)
}

func (rfq *ComputeRFQ) decorate(viewerUserId int) {
	rfq.Code = fmt.Sprintf("RFQ-%d", rfq.Id)
	rfq.BuyerAlias = ComputeMarketAlias("B", rfq.UserId)
	rfq.Mine = rfq.UserId == viewerUserId
}

// Insert creates a new RFQ in matching state with a 24h window.
func (rfq *ComputeRFQ) Insert() error {
	now := common.GetTimestamp()
	rfq.CreatedTime = now
	rfq.UpdatedTime = now
	if rfq.Status == "" {
		rfq.Status = ComputeRFQStatusMatching
	}
	if rfq.MatchingDeadline == 0 {
		rfq.MatchingDeadline = now + ComputeMarketMatchingWindowSeconds
	}
	if err := DB.Create(rfq).Error; err != nil {
		return err
	}
	rfq.decorate(rfq.UserId)
	return nil
}

func (rfq *ComputeRFQ) save() error {
	rfq.UpdatedTime = common.GetTimestamp()
	return DB.Model(rfq).Select("status", "matching_deadline", "extensions", "accepted_bid_id", "price_ceiling", "updated_time").Updates(rfq).Error
}

// GetComputeRFQById loads one RFQ, refreshing its matching/choosing state.
func GetComputeRFQById(id int, viewerUserId int) (*ComputeRFQ, error) {
	var rfq ComputeRFQ
	if err := DB.First(&rfq, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := refreshComputeRFQStatus(&rfq); err != nil {
		return nil, err
	}
	rfq.decorate(viewerUserId)
	return &rfq, nil
}

// refreshComputeRFQStatus moves an expired matching RFQ into choosing.
func refreshComputeRFQStatus(rfq *ComputeRFQ) error {
	if rfq.Status == ComputeRFQStatusMatching && common.GetTimestamp() > rfq.MatchingDeadline {
		rfq.Status = ComputeRFQStatusChoosing
		return rfq.save()
	}
	return nil
}

// ListComputeRFQsByUser returns the buyer's own requests, newest first.
func ListComputeRFQsByUser(userId int) ([]*ComputeRFQ, error) {
	var rfqs []*ComputeRFQ
	if err := DB.Where("user_id = ?", userId).Order("id desc").Find(&rfqs).Error; err != nil {
		return nil, err
	}
	for _, r := range rfqs {
		if err := refreshComputeRFQStatus(r); err != nil {
			return nil, err
		}
		r.decorate(userId)
	}
	return rfqs, nil
}

// ListOpenComputeRFQs returns requests suppliers may bid on (matching or
// choosing), soonest deadline first. Buyer identity is aliased.
func ListOpenComputeRFQs(viewerUserId int) ([]*ComputeRFQ, error) {
	var rfqs []*ComputeRFQ
	err := DB.Where("status in ?", []string{ComputeRFQStatusMatching, ComputeRFQStatusChoosing}).
		Order("featured desc, matching_deadline asc").Find(&rfqs).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rfqs {
		if err := refreshComputeRFQStatus(r); err != nil {
			return nil, err
		}
		r.decorate(viewerUserId)
	}
	return rfqs, nil
}

// ListRecentlyMatchedComputeRFQs returns requests matched/contracted/live
// since the given timestamp (for the marketplace "matched this week" strip).
func ListRecentlyMatchedComputeRFQs(viewerUserId int, since int64) ([]*ComputeRFQ, error) {
	var rfqs []*ComputeRFQ
	err := DB.Where("status in ? and updated_time >= ?", []string{ComputeRFQStatusMatched, ComputeRFQStatusContracted, ComputeRFQStatusLive}, since).
		Order("updated_time desc").Limit(50).Find(&rfqs).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rfqs {
		r.decorate(viewerUserId)
	}
	return rfqs, nil
}

// ListComputeBidsByRFQ returns all bids on an RFQ ordered by price, with
// aliases, eligibility rank and the viewer's own bid flagged.
func ListComputeBidsByRFQ(rfqId int, viewerUserId int) ([]*ComputeBid, error) {
	var bids []*ComputeBid
	if err := DB.Where("rfq_id = ?", rfqId).Order("price_per_gpu_hour asc, id asc").Find(&bids).Error; err != nil {
		return nil, err
	}
	levels := map[int]int{}
	for _, b := range bids {
		if _, ok := levels[b.SupplierUserId]; !ok {
			if s, err := GetComputeSupplierByUserId(b.SupplierUserId); err == nil {
				levels[b.SupplierUserId] = s.Level
			}
		}
	}
	decorateComputeBids(bids, viewerUserId, levels)
	return bids, nil
}

func decorateComputeBids(bids []*ComputeBid, viewerUserId int, levels map[int]int) {
	rank := 0
	for _, b := range bids {
		b.SupplierAlias = ComputeMarketAlias("S", b.SupplierUserId)
		b.SupplierLevel = levels[b.SupplierUserId]
		b.Mine = b.SupplierUserId == viewerUserId
		b.Rank = 0
		if b.Eligible && (b.Status == ComputeBidStatusLive || b.Status == ComputeBidStatusAccepted) {
			rank++
			b.Rank = rank
		}
	}
}

// TopEligibleComputeBids returns the n lowest live eligible bids.
func TopEligibleComputeBids(bids []*ComputeBid, n int) []*ComputeBid {
	out := make([]*ComputeBid, 0, n)
	sorted := append([]*ComputeBid(nil), bids...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PricePerGpuHour < sorted[j].PricePerGpuHour })
	for _, b := range sorted {
		if b.Eligible && b.Status == ComputeBidStatusLive {
			out = append(out, b)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// LowestEligibleComputeBidPrice returns the lowest live eligible price, or 0.
func LowestEligibleComputeBidPrice(bids []*ComputeBid) float64 {
	top := TopEligibleComputeBids(bids, 1)
	if len(top) == 0 {
		return 0
	}
	return top[0].PricePerGpuHour
}

// ListComputeBidsBySupplier returns a supplier's bids across RFQs.
func ListComputeBidsBySupplier(userId int) ([]*ComputeBid, error) {
	var bids []*ComputeBid
	if err := DB.Where("supplier_user_id = ?", userId).Order("id desc").Find(&bids).Error; err != nil {
		return nil, err
	}
	levels := map[int]int{}
	if s, err := GetComputeSupplierByUserId(userId); err == nil {
		levels[userId] = s.Level
	}
	for _, b := range bids {
		b.SupplierAlias = ComputeMarketAlias("S", b.SupplierUserId)
		b.SupplierLevel = levels[userId]
		b.Mine = true
	}
	return bids, nil
}

// PlaceComputeBid creates or lowers a supplier's bid on an RFQ. Bids only go
// down; a bid inside the last 15 minutes extends the matching window.
func PlaceComputeBid(rfq *ComputeRFQ, supplierUserId int, price float64, deliverDate string, slaPct float64, hardReqs string, deviations string, eligible bool) (*ComputeBid, error) {
	if rfq.Status != ComputeRFQStatusMatching && rfq.Status != ComputeRFQStatusChoosing {
		return nil, ErrComputeRFQNotMatching
	}
	if price <= 0 || price >= rfq.PriceCeiling {
		return nil, ErrComputeBidTooHigh
	}
	now := common.GetTimestamp()
	var bid ComputeBid
	err := DB.Transaction(func(tx *gorm.DB) error {
		existing := ComputeBid{}
		findErr := tx.Where("rfq_id = ? and supplier_user_id = ? and status = ?", rfq.Id, supplierUserId, ComputeBidStatusLive).First(&existing).Error
		if findErr == nil {
			if price > existing.PricePerGpuHour-ComputeMarketMinDecrement+1e-9 {
				return ErrComputeBidNotLower
			}
			existing.PricePerGpuHour = price
			existing.DeliverDate = deliverDate
			existing.SlaPct = slaPct
			existing.HardReqs = hardReqs
			existing.Deviations = deviations
			existing.Eligible = eligible
			existing.UpdatedTime = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			bid = existing
		} else if errors.Is(findErr, gorm.ErrRecordNotFound) {
			bid = ComputeBid{
				RfqId: rfq.Id, SupplierUserId: supplierUserId, PricePerGpuHour: price, DeliverDate: deliverDate,
				SlaPct: slaPct, HardReqs: hardReqs, Deviations: deviations, Eligible: eligible,
				Status: ComputeBidStatusLive, CreatedTime: now, UpdatedTime: now,
			}
			if err := tx.Create(&bid).Error; err != nil {
				return err
			}
		} else {
			return findErr
		}
		// Anti-snipe: a bid in the last 15 minutes of matching extends it.
		if rfq.Status == ComputeRFQStatusMatching && rfq.MatchingDeadline-now < ComputeMarketAntiSnipeSeconds {
			rfq.MatchingDeadline = now + ComputeMarketAntiSnipeSeconds
			rfq.UpdatedTime = now
			if err := tx.Model(rfq).Select("matching_deadline", "updated_time").Updates(rfq).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	bid.SupplierAlias = ComputeMarketAlias("S", supplierUserId)
	bid.Mine = true
	return &bid, nil
}

// WithdrawComputeBid withdraws a supplier's live bid.
func WithdrawComputeBid(bidId int, supplierUserId int) error {
	res := DB.Model(&ComputeBid{}).
		Where("id = ? and supplier_user_id = ? and status = ?", bidId, supplierUserId, ComputeBidStatusLive).
		Updates(map[string]any{"status": ComputeBidStatusWithdrawn, "updated_time": common.GetTimestamp()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrComputeBidNotAcceptable
	}
	return nil
}

// AcceptComputeBid lets the buyer lock a bid. While matching any live eligible
// bid may be accepted; once choosing, only the three lowest eligible bids.
func AcceptComputeBid(rfq *ComputeRFQ, bidId int, buyerUserId int) (*ComputeBid, error) {
	if rfq.UserId != buyerUserId {
		return nil, ErrComputeRFQNotOwner
	}
	if rfq.Status != ComputeRFQStatusMatching && rfq.Status != ComputeRFQStatusChoosing {
		return nil, ErrComputeRFQNotMatching
	}
	bids, err := ListComputeBidsByRFQ(rfq.Id, buyerUserId)
	if err != nil {
		return nil, err
	}
	var target *ComputeBid
	for _, b := range bids {
		if b.Id == bidId {
			target = b
		}
	}
	if target == nil || !target.Eligible || target.Status != ComputeBidStatusLive {
		return nil, ErrComputeBidNotAcceptable
	}
	if rfq.Status == ComputeRFQStatusChoosing {
		inTop := false
		for _, b := range TopEligibleComputeBids(bids, ComputeMarketTopN) {
			if b.Id == bidId {
				inTop = true
			}
		}
		if !inTop {
			return nil, ErrComputeBidNotAcceptable
		}
	}
	now := common.GetTimestamp()
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ComputeBid{}).Where("rfq_id = ? and status = ? and id <> ?", rfq.Id, ComputeBidStatusLive, bidId).
			Updates(map[string]any{"status": ComputeBidStatusRejected, "updated_time": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&ComputeBid{}).Where("id = ?", bidId).
			Updates(map[string]any{"status": ComputeBidStatusAccepted, "updated_time": now}).Error; err != nil {
			return err
		}
		rfq.Status = ComputeRFQStatusMatched
		rfq.AcceptedBidId = bidId
		rfq.UpdatedTime = now
		return tx.Model(rfq).Select("status", "accepted_bid_id", "updated_time").Updates(rfq).Error
	})
	if err != nil {
		return nil, err
	}
	target.Status = ComputeBidStatusAccepted
	return target, nil
}

// ExtendComputeRFQ gives a request another 24h of matching (max 3 times).
func ExtendComputeRFQ(rfq *ComputeRFQ, buyerUserId int) error {
	if rfq.UserId != buyerUserId {
		return ErrComputeRFQNotOwner
	}
	if rfq.Status != ComputeRFQStatusMatching && rfq.Status != ComputeRFQStatusChoosing {
		return ErrComputeRFQNotMatching
	}
	if rfq.Extensions >= ComputeMarketMaxExtensions {
		return ErrComputeRFQMaxExtensions
	}
	now := common.GetTimestamp()
	base := rfq.MatchingDeadline
	if base < now {
		base = now
	}
	rfq.MatchingDeadline = base + ComputeMarketMatchingWindowSeconds
	rfq.Extensions++
	rfq.Status = ComputeRFQStatusMatching
	return rfq.save()
}

// RaiseComputeRFQCeiling raises the public price ceiling (never lowers it).
func RaiseComputeRFQCeiling(rfq *ComputeRFQ, buyerUserId int, ceiling float64) error {
	if rfq.UserId != buyerUserId {
		return ErrComputeRFQNotOwner
	}
	if rfq.Status != ComputeRFQStatusMatching && rfq.Status != ComputeRFQStatusChoosing {
		return ErrComputeRFQNotMatching
	}
	if ceiling <= rfq.PriceCeiling {
		return ErrComputeCeilingNotHigher
	}
	rfq.PriceCeiling = ceiling
	return rfq.save()
}

// CancelComputeRFQ withdraws an un-matched request.
func CancelComputeRFQ(rfq *ComputeRFQ, buyerUserId int) error {
	if rfq.UserId != buyerUserId {
		return ErrComputeRFQNotOwner
	}
	if rfq.Status != ComputeRFQStatusMatching && rfq.Status != ComputeRFQStatusChoosing {
		return ErrComputeRFQNotMatching
	}
	now := common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ComputeBid{}).Where("rfq_id = ? and status = ?", rfq.Id, ComputeBidStatusLive).
			Updates(map[string]any{"status": ComputeBidStatusRejected, "updated_time": now}).Error; err != nil {
			return err
		}
		rfq.Status = ComputeRFQStatusCancelled
		rfq.UpdatedTime = now
		return tx.Model(rfq).Select("status", "updated_time").Updates(rfq).Error
	})
}

// ---- Supplier profiles ----

func GetComputeSupplierByUserId(userId int) (*ComputeSupplier, error) {
	var s ComputeSupplier
	if err := DB.First(&s, "user_id = ?", userId).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertComputeSupplier creates or updates the caller's supplier profile.
// Level is never changed here (admin-only).
func UpsertComputeSupplier(userId int, company, contact, regions, inventory string) (*ComputeSupplier, error) {
	now := common.GetTimestamp()
	existing, err := GetComputeSupplierByUserId(userId)
	if err == nil {
		existing.Company, existing.Contact, existing.Regions, existing.Inventory = company, contact, regions, inventory
		existing.UpdatedTime = now
		if err := DB.Model(existing).Select("company", "contact", "regions", "inventory", "updated_time").Updates(existing).Error; err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	s := &ComputeSupplier{UserId: userId, Company: company, Contact: contact, Regions: regions, Inventory: inventory,
		Level: ComputeSupplierLevelRegistered, CreatedTime: now, UpdatedTime: now}
	if err := DB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

func ListComputeSuppliers() ([]*ComputeSupplier, error) {
	var out []*ComputeSupplier
	err := DB.Order("id desc").Find(&out).Error
	return out, err
}

func SetComputeSupplierLevel(userId int, level int) error {
	if level < ComputeSupplierLevelRegistered || level > ComputeSupplierLevelPreferred {
		return errors.New("level must be 0, 1 or 2")
	}
	res := DB.Model(&ComputeSupplier{}).Where("user_id = ?", userId).
		Updates(map[string]any{"level": level, "updated_time": common.GetTimestamp()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ComputeRFQRegionsList splits the stored comma list.
func ComputeRFQRegionsList(regions string) []string {
	var out []string
	for _, r := range strings.Split(regions, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// SetComputeRFQFeatured pins or unpins a request on the marketplace (admin).
func SetComputeRFQFeatured(id int, featured bool) error {
	res := DB.Model(&ComputeRFQ{}).Where("id = ?", id).Updates(map[string]any{"featured": featured, "updated_time": common.GetTimestamp()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

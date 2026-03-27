package compute

// driver_compute.go — Business Driver Framework, Phase 2.
//
// For products with a typed DriverType, ApplyDriverCompute derives Volume
// and per-unit economics from DriverParams, overriding the stored
// ProductAssumption cost fields and ProductSalesVolume records before
// the standard ComputeProductRevenue engine runs.
//
// Design principle
// ────────────────
//   generic     → pass-through (stored assumptions + volumes unchanged)
//   consulting  → derive billable-day volume + cost/day; user sets billing rate
//   saas        → derive user volume + annual unit price + annual unit cost
//   industry    → keep user-set volume; adjust unit cost for scrap & setup
//   marketplace → derive transaction volume + net revenue/tx + variable cost/tx
//   media       → derive per-mille volume + CPM revenue + delivery+content cost
//
// All generated volumes are placed in ZoneFrance/ChannelDirect.  Existing
// distributor margins are always preserved unmodified.

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"ascenda/internal/model"
)

// ApplyDriverCompute returns a ProductInputBundle with volumes and assumptions
// overridden according to the product's DriverType and DriverParams.
// For DriverGeneric (or empty) the bundle is returned unchanged.
// On parse error the original bundle is returned along with the error so the
// caller can decide whether to hard-fail or soft-warn.
func ApplyDriverCompute(product model.Product, bundle ProductInputBundle) (ProductInputBundle, error) {
	dt := product.DriverType
	if dt == "" || dt == model.DriverGeneric {
		return bundle, nil
	}
	if len(product.DriverParams) == 0 {
		// Params not yet configured — fall back to generic.
		return bundle, nil
	}

	switch dt {
	case model.DriverConsulting:
		var p model.ConsultingParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("consulting params unmarshal: %w", err)
		}
		return applyConsultingDriver(p, bundle, product.ID), nil

	case model.DriverSaaS:
		var p model.SaaSParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("saas params unmarshal: %w", err)
		}
		return applySaaSDriver(p, bundle, product.ID), nil

	case model.DriverIndustry:
		var p model.IndustryParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("industry params unmarshal: %w", err)
		}
		return applyIndustryDriver(p, bundle), nil

	case model.DriverMarketplace:
		var p model.MarketplaceParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("marketplace params unmarshal: %w", err)
		}
		return applyMarketplaceDriver(p, bundle, product.ID), nil

	case model.DriverMedia:
		var p model.MediaParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("media params unmarshal: %w", err)
		}
		return applyMediaDriver(p, bundle, product.ID), nil

	case model.DriverSessionBased:
		var p model.SessionBasedParams
		if err := json.Unmarshal(product.DriverParams, &p); err != nil {
			return bundle, fmt.Errorf("session_based params unmarshal: %w", err)
		}
		return applySessionBasedDriver(p, bundle, product.ID), nil

	default:
		return bundle, nil
	}
}

// ---------------------------------------------------------------------------
// Consulting driver
//
//   BillableDays[y] = Headcount[y] × WorkingDays × UtilizationRate[y]
//   Revenue[y]      = BillableDays[y] × BaseUnitPrice[y]   (billing rate, user-set)
//   CostPerDay[y]   = (Headcount[y] × MonthlyGross[y] × 12 × EmployerCharges)
//                     / BillableDays[y]
// ---------------------------------------------------------------------------

func applyConsultingDriver(params model.ConsultingParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	wd := decimal.NewFromInt(int64(params.WorkingDays))
	twelve := decimal.NewFromInt(12)
	one := decimal.NewFromInt(1)

	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		// Billable days for the whole team in this year.
		billableDays := params.Headcount[y].Mul(wd).Mul(params.UtilizationRate[y])
		days := billableDays.IntPart() // truncate to whole days

		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: days,
		})

		// CostPerDay = (FTE × salary × 12 × employer factor) / team billable days
		annualTeamCost := params.Headcount[y].
			Mul(params.MonthlyGross[y]).
			Mul(twelve).
			Mul(params.EmployerCharges)

		costPerDay := SafeDiv(annualTeamCost, billableDays)

		// Override cost assumptions; keep BaseUnitPrice (billing rate is user-set).
		bundle.Assumptions[y].RawMaterialCost = costPerDay
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

// ---------------------------------------------------------------------------
// SaaS driver
//
//   Volume[y]    = ActiveUsers[y]
//   UnitPrice[y] = MonthlyFee[y] × 12            (annual revenue per user)
//   UnitCost[y]  = (InfraCostPPU[y] + SupportCostPPU[y]) × 12
// ---------------------------------------------------------------------------

func applySaaSDriver(params model.SaaSParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	twelve := decimal.NewFromInt(12)
	one := decimal.NewFromInt(1)

	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: int64(params.ActiveUsers[y]),
		})

		annualFee := params.MonthlyFee[y].Mul(twelve)
		annualCostPerUser := params.InfraCostPPU[y].
			Add(params.SupportCostPPU[y]).
			Mul(twelve)

		bundle.Assumptions[y].BaseUnitPrice = annualFee
		bundle.Assumptions[y].RawMaterialCost = annualCostPerUser
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

// ---------------------------------------------------------------------------
// Industry driver
//
//   Volume      = stored ProductSalesVolume (user-set demand forecast)
//   UnitPrice   = stored BaseUnitPrice       (selling price, user-set)
//   UnitCost[y] = storedGrossCost / (1 − ScrapRate[y])
//                 + SetupCost[y] / TotalUnitSales[y]
//
// ScrapRate raises effective production cost because more units must be
// manufactured to deliver the sold quantity.  SetupCost is a fixed annual
// tooling/batch cost amortised across total unit sales.
// ---------------------------------------------------------------------------

func applyIndustryDriver(params model.IndustryParams, bundle ProductInputBundle) ProductInputBundle {
	// Accumulate total units sold per year for setup cost amortisation.
	unitsSoldByYear := [MaxYears]int64{}
	for _, v := range bundle.Volumes {
		if v.YearIndex >= 1 && v.YearIndex <= MaxYears {
			unitsSoldByYear[v.YearIndex-1] += v.UnitsSold
		}
	}

	one := decimal.NewFromInt(1)

	// cumulativeInventoryUnits tracks the running finished-goods stock count.
	// It starts at zero (no opening finished-goods inventory at plan start) and
	// is adjusted each year by (UnitsProduced - UnitsSold).  When UnitsProduced
	// is zero for a year we treat it as "produce-to-order" (no surplus/deficit).
	cumulativeInventoryUnits := int64(0)

	for y := 0; y < MaxYears; y++ {
		// Gross cost from stored assumption (material + royalties + logistics × coeff).
		grossCost := bundle.Assumptions[y].RawMaterialCost.
			Add(bundle.Assumptions[y].RoyaltiesCost).
			Add(bundle.Assumptions[y].LogisticsCost).
			Mul(bundle.Assumptions[y].CostCoefficient)

		// Scrap adjustment: EffectiveCost = grossCost / (1 - ScrapRate).
		scrapDivisor := one.Sub(params.ScrapRate[y])
		if scrapDivisor.LessThanOrEqual(decimal.Zero) {
			scrapDivisor = one // guard against bad inputs
		}
		effectiveCost := grossCost.Div(scrapDivisor)

		// Amortise annual setup cost per unit sold.
		totalUnits := decimal.NewFromInt(unitsSoldByYear[y])
		setupPerUnit := SafeDiv(params.SetupCost[y], totalUnits)
		effectiveCost = effectiveCost.Add(setupPerUnit)

		// Flatten to a single RawMaterialCost with coeff = 1.
		bundle.Assumptions[y].RawMaterialCost = effectiveCost
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		// PriceCoefficient preserved — user may set a year-over-year price ramp.

		// ── Finished-goods surplus inventory ────────────────────────────────────
		// When the user has specified UnitsProduced[y] > 0 (meaning they are
		// managing a production plan that may differ from sales volume), compute
		// the cumulative finished-goods inventory at end of year y and its cost.
		// If UnitsProduced[y] == 0 we interpret it as produce-to-order, leaving
		// SurplusInventoryValue unchanged (stays zero or carries forward).
		if params.UnitsProduced[y] > 0 {
			cumulativeInventoryUnits += int64(params.UnitsProduced[y]) - unitsSoldByYear[y]
			if cumulativeInventoryUnits < 0 {
				cumulativeInventoryUnits = 0 // cannot draw below zero
			}
			bundle.SurplusInventoryValue[y] = decimal.NewFromInt(cumulativeInventoryUnits).
				Mul(effectiveCost)
		}
		// If produce-to-order (UnitsProduced == 0), SurplusInventoryValue[y] keeps
		// its zero value — no finished-goods inventory on the balance sheet.
	}

	return bundle
}

// ---------------------------------------------------------------------------
// Marketplace driver
//
//   Volume[y]    = Transactions[y]
//   UnitPrice[y] = GMVPerTransaction[y] × TakeRate[y]       (net platform revenue per tx)
//   UnitCost[y]  = PaymentCost[y] + FixedInfraCost[y] / Transactions[y]
// ---------------------------------------------------------------------------

func applyMarketplaceDriver(params model.MarketplaceParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	one := decimal.NewFromInt(1)
	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		tx := int64(params.Transactions[y])
		txDec := decimal.NewFromInt(tx)

		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: tx,
		})

		netRevenuePerTx := params.GMVPerTransaction[y].Mul(params.TakeRate[y])
		fixedPerTx := SafeDiv(params.FixedInfraCost[y], txDec)
		costPerTx := params.PaymentCost[y].Add(fixedPerTx)

		bundle.Assumptions[y].BaseUnitPrice = netRevenuePerTx
		bundle.Assumptions[y].RawMaterialCost = costPerTx
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

// ---------------------------------------------------------------------------
// Media driver
//
//   Volume[y]    = Impressions[y] / 1000          (per-mille blocks)
//   UnitPrice[y] = CPM[y] × FillRate[y]           (revenue per 1 000 impressions)
//   UnitCost[y]  = DeliveryCostPerImpression[y] × 1 000
//                  + ContentCost[y] / (Impressions[y] / 1 000)   (amortised)
// ---------------------------------------------------------------------------

func applyMediaDriver(params model.MediaParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	thousand := decimal.NewFromInt(1000)
	one := decimal.NewFromInt(1)
	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		perMilleBlocks := int64(params.Impressions[y]) / 1000
		perMilleDec := decimal.NewFromInt(perMilleBlocks)

		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: perMilleBlocks,
		})

		revenuePerMille := params.CPM[y].Mul(params.FillRate[y])
		deliveryPerMille := params.DeliveryCostPerImpression[y].Mul(thousand)
		contentPerMille := SafeDiv(params.ContentCost[y], perMilleDec)
		costPerMille := deliveryPerMille.Add(contentPerMille)

		bundle.Assumptions[y].BaseUnitPrice = revenuePerMille
		bundle.Assumptions[y].RawMaterialCost = costPerMille
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

// ---------------------------------------------------------------------------
// Session-Based driver  (training, events, workshops)
//
//   TrainerCapacity[y]      = TrainerCount[y] × SessionsPerTrainer[y] × UtilizationRate[y]
//   ActualSessions[y]       = min(Sessions[y], TrainerCapacity[y])      ← capacity clamp
//   RealizedParticipants[y] = ParticipantsPerSession[y] × FillRate[y]
//   UnitPrice[y]            = RealizedParticipants[y] × PricePerParticipant[y]
//   UnitCost[y]             = TrainerCostPerSession[y]
//                             + VariableCostPerParticipant[y] × RealizedParticipants[y]
//   Volume[y]               = ActualSessions[y]
// ---------------------------------------------------------------------------

func applySessionBasedDriver(params model.SessionBasedParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	one := decimal.NewFromInt(1)
	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		// ── Capacity clamp ────────────────────────────────────────────────────
		trainerCapacity := params.TrainerCount[y].
			Mul(decimal.NewFromInt(int64(params.SessionsPerTrainer[y]))).
			Mul(params.UtilizationRate[y])

		// clamp: ActualSessions = min(Sessions, TrainerCapacity)
		plannedDec := decimal.NewFromInt(int64(params.Sessions[y]))
		actualSessionsDec := plannedDec
		if trainerCapacity.LessThan(plannedDec) {
			actualSessionsDec = trainerCapacity
		}
		actualSessions := actualSessionsDec.IntPart()

		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: actualSessions,
		})

		// ── Per-session economics ──────────────────────────────────────────────
		realizedParticipants := params.ParticipantsPerSession[y].Mul(params.FillRate[y])

		// UnitPrice = participants × price-per-participant
		unitPrice := realizedParticipants.Mul(params.PricePerParticipant[y])

		// UnitCost = fixed trainer/venue cost + variable per-participant cost
		unitCost := params.TrainerCostPerSession[y].
			Add(params.VariableCostPerParticipant[y].Mul(realizedParticipants))

		bundle.Assumptions[y].BaseUnitPrice = unitPrice
		bundle.Assumptions[y].RawMaterialCost = unitCost
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

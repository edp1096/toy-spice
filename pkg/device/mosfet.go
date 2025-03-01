package device

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/matrix"
)

// Mosfet Levels 1-3 implementation
type Mosfet struct {
	BaseDevice
	Type  string // "NMOS" or "PMOS"
	Level int    // Model level (1-3)

	// Geometry parameters
	L   float64 // Channel length (m)
	W   float64 // Channel width (m)
	AD  float64 // Drain area (m²)
	AS  float64 // Source area (m²)
	PD  float64 // Drain perimeter (m)
	PS  float64 // Source perimeter (m)
	NRD float64 // Drain squares
	NRS float64 // Source squares

	// DC Parameters - Common
	VTO    float64 // Threshold voltage
	KP     float64 // Transconductance parameter (A/V²)
	GAMMA  float64 // Body effect parameter (V^0.5)
	PHI    float64 // Surface potential (V)
	LAMBDA float64 // Channel length modulation (1/V)
	RD     float64 // Drain resistance (Ω)
	RS     float64 // Source resistance (Ω)
	RSH    float64 // Sheet resistance (Ω/□)
	IS     float64 // Bulk junction saturation current (A)
	JS     float64 // Bulk junction saturation current density (A/m²)
	N      float64 // Bulk junction emission coefficient

	// Capacitance Parameters
	CBD  float64 // Bulk-Drain zero-bias capacitance (F)
	CBS  float64 // Bulk-Source zero-bias capacitance (F)
	CGSO float64 // Gate-Source overlap capacitance per unit width (F/m)
	CGDO float64 // Gate-Drain overlap capacitance per unit width (F/m)
	CGBO float64 // Gate-Bulk overlap capacitance per unit length (F/m)
	CJ   float64 // Bulk junction capacitance (F/m²)
	MJ   float64 // Bulk junction grading coefficient
	CJSW float64 // Bulk junction sidewall capacitance (F/m)
	MJSW float64 // Bulk junction sidewall grading coefficient
	PB   float64 // Bulk junction potential (V)
	FC   float64 // Forward-bias depletion capacitance coefficient

	// Level 2 Parameters
	TOX   float64 // Oxide thickness (m)
	NSUB  float64 // Substrate doping (1/cm³)
	NSS   float64 // Surface state density (1/cm²)
	NFS   float64 // Fast surface state density (1/cm²)
	TPG   float64 // Gate material type: +1: opposite of substrate, -1: same as substrate, 0: aluminum
	XJ    float64 // Metallurgical junction depth (m)
	LD    float64 // Lateral diffusion (m)
	UO    float64 // Surface mobility (cm²/V·s)
	UCRIT float64 // Critical field for mobility degradation (V/cm)
	UEXP  float64 // Critical field exponent
	UTRA  float64 // Transverse field coefficient
	VMAX  float64 // Maximum drift velocity (m/s)
	NEFF  float64 // Channel charge coefficient
	XQC   float64 // Charge-based model flag

	// Level 3 Parameters
	DELTA float64 // Width effect on threshold voltage
	THETA float64 // Mobility modulation
	ETA   float64 // Static feedback
	KAPPA float64 // Saturation field factor

	// Temperature Parameters
	TNOM float64 // Parameter measurement temperature (K)
	KF   float64 // Flicker noise coefficient
	AF   float64 // Flicker noise exponent

	// Internal states
	vgs float64 // Gate-Source voltage
	vds float64 // Drain-Source voltage
	vbs float64 // Bulk-Source voltage
	vgd float64 // Gate-Drain voltage
	vbd float64 // Bulk-Drain voltage

	id   float64 // Drain current
	gm   float64 // Transconductance
	gds  float64 // Drain-Source conductance
	gmbs float64 // Body-effect transconductance
	cgs  float64 // Gate-Source capacitance
	cgd  float64 // Gate-Drain capacitance
	cgb  float64 // Gate-Bulk capacitance

	// Operation region
	region int // 0: cutoff, 1: linear, 2: saturation

	// Previous states for transient
	prevVgs float64
	prevVds float64
	prevVbs float64
	prevId  float64

	// Charge storage
	qgs float64 // Gate-Source charge
	qgd float64 // Gate-Drain charge
	qgb float64 // Gate-Bulk charge
	qbs float64 // Bulk-Source charge
	qbd float64 // Bulk-Drain charge

	// Previous charge storage
	prevQgs float64
	prevQgd float64
	prevQgb float64
	prevQbs float64
	prevQbd float64
}

const (
	CUTOFF     = 0 // Cutoff region
	LINEAR     = 1 // Linear/Triode region
	SATURATION = 2 // Saturation region
)

func NewMosfet(name string, nodeNames []string) *Mosfet {
	if len(nodeNames) != 4 {
		panic(fmt.Sprintf("mosfet %s: requires exactly 4 nodes (drain, gate, source, bulk)", name))
	}

	m := &Mosfet{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
		Type:  "NMOS", // Default NMOS
		Level: 1,      // Default level 1
	}
	m.setDefaultParameters()
	return m
}

func (m *Mosfet) GetType() string { return "M" }

func (m *Mosfet) setDefaultParameters() {
	// Geometry defaults
	m.L = 10e-6 // 10 μm
	m.W = 10e-6 // 10 μm
	m.AD = 0.0
	m.AS = 0.0
	m.PD = 0.0
	m.PS = 0.0
	m.NRD = 1.0
	m.NRS = 1.0

	// Common DC parameters
	m.VTO = 0.7     // Threshold voltage
	m.KP = 2e-5     // Transconductance parameter
	m.GAMMA = 0.5   // Body effect parameter
	m.PHI = 0.6     // Surface potential
	m.LAMBDA = 0.01 // Channel length modulation
	m.RD = 0.0      // Drain resistance
	m.RS = 0.0      // Source resistance
	m.RSH = 0.0     // Sheet resistance
	m.IS = 1e-14    // Bulk junction saturation current
	m.JS = 0.0      // Bulk junction saturation current density
	m.N = 1.0       // Bulk junction emission coefficient

	// Capacitance defaults
	m.CBD = 0.0   // Bulk-Drain zero-bias capacitance
	m.CBS = 0.0   // Bulk-Source zero-bias capacitance
	m.CGSO = 0.0  // Gate-Source overlap capacitance per unit width
	m.CGDO = 0.0  // Gate-Drain overlap capacitance per unit width
	m.CGBO = 0.0  // Gate-Bulk overlap capacitance per unit length
	m.CJ = 0.0    // Bulk junction capacitance
	m.MJ = 0.5    // Bulk junction grading coefficient
	m.CJSW = 0.0  // Bulk junction sidewall capacitance
	m.MJSW = 0.33 // Bulk junction sidewall grading coefficient
	m.PB = 0.8    // Bulk junction potential
	m.FC = 0.5    // Forward-bias depletion capacitance coefficient

	// Level 2 parameters
	m.TOX = 1e-7  // Oxide thickness (100 nm)
	m.NSUB = 1e16 // Substrate doping
	m.NSS = 0.0   // Surface state density
	m.NFS = 0.0   // Fast surface state density
	m.TPG = 1.0   // Gate material type
	m.XJ = 0.0    // Metallurgical junction depth
	m.LD = 0.0    // Lateral diffusion
	m.UO = 600.0  // Surface mobility
	m.UCRIT = 1e4 // Critical field for mobility degradation
	m.UEXP = 0.0  // Critical field exponent
	m.UTRA = 0.0  // Transverse field coefficient
	m.VMAX = 0.0  // Maximum drift velocity
	m.NEFF = 1.0  // Channel charge coefficient
	m.XQC = 0.6   // Charge-based model flag

	// Level 3 parameters
	m.DELTA = 0.0 // Width effect on threshold voltage
	m.THETA = 0.0 // Mobility modulation
	m.ETA = 0.0   // Static feedback
	m.KAPPA = 0.2 // Saturation field factor

	// Temperature parameters
	m.TNOM = 300.15 // 27°C
	m.KF = 0.0      // Flicker noise coefficient
	m.AF = 1.0      // Flicker noise exponent
}

func (m *Mosfet) SetModelParameters(params map[string]float64) {
	if levelVal, ok := params["level"]; ok {
		m.Level = int(levelVal)
	}

	if typeVal, ok := params["type"]; ok {
		if typeVal == 1.0 {
			m.Type = "PMOS"
		} else {
			m.Type = "NMOS"
		}
	}

	paramsSet := map[string]*float64{
		// Geometry parameters
		"l":   &m.L,
		"w":   &m.W,
		"ad":  &m.AD,
		"as":  &m.AS,
		"pd":  &m.PD,
		"ps":  &m.PS,
		"nrd": &m.NRD,
		"nrs": &m.NRS,

		// Common DC parameters
		"vto":    &m.VTO,
		"kp":     &m.KP,
		"gamma":  &m.GAMMA,
		"phi":    &m.PHI,
		"lambda": &m.LAMBDA,
		"rd":     &m.RD,
		"rs":     &m.RS,
		"rsh":    &m.RSH,
		"is":     &m.IS,
		"js":     &m.JS,
		"n":      &m.N,

		// Capacitance parameters
		"cbd":  &m.CBD,
		"cbs":  &m.CBS,
		"cgso": &m.CGSO,
		"cgdo": &m.CGDO,
		"cgbo": &m.CGBO,
		"cj":   &m.CJ,
		"mj":   &m.MJ,
		"cjsw": &m.CJSW,
		"mjsw": &m.MJSW,
		"pb":   &m.PB,
		"fc":   &m.FC,

		// Level 2 specific parameters
		"tox":   &m.TOX,
		"nsub":  &m.NSUB,
		"nss":   &m.NSS,
		"nfs":   &m.NFS,
		"tpg":   &m.TPG,
		"xj":    &m.XJ,
		"ld":    &m.LD,
		"uo":    &m.UO,
		"ucrit": &m.UCRIT,
		"uexp":  &m.UEXP,
		"utra":  &m.UTRA,
		"vmax":  &m.VMAX,
		"neff":  &m.NEFF,
		"xqc":   &m.XQC,

		// Level 3 specific parameters
		"delta": &m.DELTA,
		"theta": &m.THETA,
		"eta":   &m.ETA,
		"kappa": &m.KAPPA,

		// Temperature parameters
		"tnom": &m.TNOM,
		"kf":   &m.KF,
		"af":   &m.AF,
	}

	for key, param := range paramsSet {
		if value, ok := params[key]; ok {
			*param = value
		}
	}
}

// Calculate threshold voltage with body effect
func (m *Mosfet) calculateVth(vbs float64) float64 {
	vt0 := m.VTO

	// Apply body effect
	if m.GAMMA > 0 {
		// GAMMA * (sqrt(PHI - VBS) - sqrt(PHI))
		vth := vt0 + m.GAMMA*(math.Sqrt(math.Max(0, m.PHI-vbs))-math.Sqrt(m.PHI))

		// For PMOS, negate the threshold voltage
		if m.Type == "PMOS" {
			vth = -vth
		}

		return vth
	}

	// For PMOS, negate the threshold voltage
	if m.Type == "PMOS" {
		return -vt0
	}

	return vt0
}

// Determine operation region and calculate drain current
func (m *Mosfet) calculateCurrents(vgs, vds, vbs, temp float64) (float64, int) {
	// Sign adjustment for PMOS
	sign := 1.0
	if m.Type == "PMOS" {
		vgs = -vgs
		vds = -vds
		vbs = -vbs
		sign = -1.0
	}

	vth := m.calculateVth(vbs) // Calculate threshold voltage with body effect
	vgst := vgs - vth          // Effective gate voltage

	// Check operation region
	if vgst <= 0 {
		return 0.0, CUTOFF // Cutoff region
	}

	// Calculate drain current based on model level
	var id float64
	var region int

	switch m.Level {
	case 1:
		id, region = m.calculateLevel1Current(vgs, vds, vbs, vth, temp)
	case 2:
		id, region = m.calculateLevel2Current(vgs, vds, vbs, vth, temp)
	case 3:
		id, region = m.calculateLevel3Current(vgs, vds, vbs, vth, temp)
	default:
		id, region = m.calculateLevel1Current(vgs, vds, vbs, vth, temp) // Fallback to Level 1
	}

	return sign * id, region // Apply sign for PMOS
}

// Level 1 (Shockley) model current calculation
func (m *Mosfet) calculateLevel1Current(vgs, vds, vbs, vth, temp float64) (float64, int) {
	// Effective gate voltage
	vgst := vgs - vth

	// Transconductance parameter
	beta := m.KP * m.W / m.L

	// Check operation region
	if vds < vgst {
		// Linear region
		id := beta * (vgst*vds - 0.5*vds*vds) * (1.0 + m.LAMBDA*vds)
		return id, LINEAR
	} else {
		// Saturation region
		id := 0.5 * beta * vgst * vgst * (1.0 + m.LAMBDA*vds)
		return id, SATURATION
	}
}

// Level 2 (Grove-Frohman) model current calculation
func (m *Mosfet) calculateLevel2Current(vgs, vds, vbs, vth, temp float64) (float64, int) {
	vgst := vgs - vth

	// Physical constants
	eps0 := 8.85e-14     // Vacuum permittivity (F/cm)
	epsox := 3.9 * eps0  // Silicon dioxide permittivity
	cox := epsox / m.TOX // Oxide capacitance

	// Fix effective electric field calculation (using vgst, unit conversion)
	eeff := vgst / (m.TOX * 100) // TOX(m) to cm

	// Mobility correction
	ueff := m.UO
	if m.UCRIT > 0 && eeff > 0 {
		ueff /= (1.0 + math.Pow(eeff/m.UCRIT, m.UEXP))
	}

	// Fix saturation voltage calculation
	vdsat := vgst
	if m.VMAX > 0 {
		// Unit conversion: VMAX(cm/s), ueff(cm²/Vs), L(m)
		ecrit := m.VMAX / ueff * 100 // V/cm
		vdsat = math.Min(vgst, ecrit*m.L)
	}

	// Calculate beta
	beta := ueff * cox * m.W / (m.L * 100) // Convert L to cm

	// Current calculation (linear/saturation region)
	var id float64
	var region int
	if vds < vdsat {
		id = beta * (vgst*vds - 0.5*vds*vds) * (1.0 + m.LAMBDA*vds)
		region = LINEAR
	} else {
		id = 0.5 * beta * vdsat * vdsat * (1.0 + m.LAMBDA*vds)
		region = SATURATION
	}

	return id, region
}

// Level 3 (Semi-empirical) model current calculation
func (m *Mosfet) calculateLevel3Current(vgs, vds, vbs, vth, temp float64) (float64, int) {
	vgst := vgs - vth

	// Mobility degradation
	vgst_eff := vgst
	if m.THETA > 0 {
		vgst_eff = vgst / (1.0 + m.THETA*vgst)
	}

	// Threshold voltage adjustment (correct use of ETA)
	vth_eff := vth
	if m.ETA > 0 {
		vth_eff += m.ETA * vds
	}

	// Calculate saturation voltage
	vdsat := vgst_eff
	if m.KAPPA > 0 {
		vdsat = vgst_eff / math.Sqrt(1.0+m.KAPPA*vgst_eff)
	}

	// Calculate beta (including channel width effect)
	beta := m.KP * m.W / m.L
	if m.DELTA > 0 {
		beta /= (1.0 + m.DELTA/m.W)
	}

	// Current calculation
	var id float64
	var region int
	if vds < vdsat {
		id = beta * (vgst_eff*vds - 0.5*vds*vds/(1.0+m.KAPPA*vgst_eff)) * (1.0 + m.LAMBDA*vds)
		region = LINEAR
	} else {
		id = 0.5 * beta * vdsat * vdsat * (1.0 + m.LAMBDA*vds)
		region = SATURATION
	}
	return id, region
}

// Calculate conductances
func (m *Mosfet) calculateConductances() {
	// Sign adjustment for PMOS
	sign := 1.0
	if m.Type == "PMOS" {
		sign = -1.0
	}

	vgs := m.vgs * sign
	vds := m.vds * sign
	vbs := m.vbs * sign

	// Calculate threshold voltage
	vth := m.calculateVth(vbs)

	// Effective gate voltage
	vgst := vgs - vth

	// Transconductance parameter
	beta := m.KP * m.W / m.L

	// Minimum conductance for numerical stability
	gmin := 1e-12

	if m.region == CUTOFF {
		// Cutoff region - minimal conductances
		m.gm = gmin
		m.gds = gmin
		m.gmbs = gmin
		return
	}

	// Body effect factor
	if m.GAMMA > 0 && m.PHI > 0 {
		if vbs < 0 {
			m.gmbs = m.gm * m.GAMMA / (2.0 * math.Sqrt(m.PHI-vbs))
		} else {
			m.gmbs = gmin
		}
	} else {
		m.gmbs = gmin
	}

	// Conductances based on model level and region
	switch m.Level {
	case 1:
		if m.region == LINEAR {
			// Linear region - Level 1
			m.gm = beta * vds * (1.0 + m.LAMBDA*vds)
			m.gds = beta*(vgst-vds)*(1.0+m.LAMBDA*vds) + beta*m.LAMBDA*(vgst*vds-0.5*vds*vds)
		} else {
			// Saturation region - Level 1
			m.gm = beta * vgst * (1.0 + m.LAMBDA*vds)
			m.gds = 0.5 * beta * vgst * vgst * m.LAMBDA
		}

	case 2, 3:
		delta := 1e-6
		id0 := m.id // Original current

		// Change in current with small change in vgs
		idg, _ := m.calculateCurrents(vgs+delta, vds, vbs, 300.15)
		m.gm = math.Max((idg-id0)/delta, gmin)

		// Change in current with small change in vds
		idd, _ := m.calculateCurrents(vgs, vds+delta, vbs, 300.15)
		m.gds = math.Max((idd-id0)/delta, gmin)

		// Change in current with small change in vbs
		idb, _ := m.calculateCurrents(vgs, vds, vbs+delta, 300.15)
		m.gmbs = math.Max((idb-id0)/delta, gmin)
	}

	// Apply sign adjustment for PMOS
	m.gm *= sign
	m.gmbs *= sign
}

// Calculate capacitances
func (m *Mosfet) calculateCapacitances() {
	// Meyer capacitance model
	cgs := 0.0
	cgd := 0.0
	cgb := 0.0

	// Gate oxide capacitance per unit area
	cox := 3.9 * 8.85e-14 / m.TOX // εox / tox
	cgate := cox * m.W * m.L      // Total gate capacitance

	// Overlap capacitances
	cgso := m.CGSO * m.W
	cgdo := m.CGDO * m.W
	cgbo := m.CGBO * m.L

	// Junction capacitances
	cbs := m.CBS
	if cbs == 0 && m.CJ > 0 {
		cbs = m.CJ*m.AS + m.CJSW*m.PS
	}

	cbd := m.CBD
	if cbd == 0 && m.CJ > 0 {
		cbd = m.CJ*m.AD + m.CJSW*m.PD
	}

	m.CBS = cbs
	m.CBD = cbd

	// Meyer capacitance model based on operation region
	switch m.region {
	case CUTOFF:
		// Cutoff region: all capacitance to bulk
		cgb = 2.0 * cgate / 3.0
		cgs = cgso
		cgd = cgdo

	case LINEAR:
		// Linear region: split between source and drain
		cgs = cgate/2.0 + cgso
		cgd = cgate/2.0 + cgdo
		cgb = cgbo

	case SATURATION:
		// Saturation region: mostly to source
		cgs = 2.0*cgate/3.0 + cgso
		cgd = cgdo
		cgb = cgbo + cgate/3.0
	}

	// Store capacitances
	m.cgs = cgs
	m.cgd = cgd
	m.cgb = cgb
}

// Calculate charges for transient analysis
func (m *Mosfet) calculateCharges() {
	switch m.region {
	case CUTOFF:
		m.qgs = 0.0
		m.qgd = 0.0
		m.qgb = m.cgb * (m.vgs - m.vbs)

	case LINEAR:
		m.qgs = m.cgs * m.vgs
		m.qgd = m.cgd * m.vgd
		m.qgb = m.cgb * (m.vgs - m.vbs)

	case SATURATION:
		m.qgs = m.cgs * m.vgs
		m.qgd = m.cgd * m.vgd
		m.qgb = m.cgb * (m.vgs - m.vbs)
	}

	var cbs, cbd float64

	// Junction capacitances with voltage dependence
	if m.vbs < 0 {
		// Reverse bias
		cbs = m.CBS / math.Pow(1.0-m.vbs/m.PB, m.MJ)
	} else {
		// Forward bias
		cbs = m.CBS * (1.0 + m.MJ*m.vbs/m.PB)
	}

	if m.vbd < 0 {
		// Reverse bias
		cbd = m.CBD / math.Pow(1.0-m.vbd/m.PB, m.MJ)
	} else {
		// Forward bias
		cbd = m.CBD * (1.0 + m.MJ*m.vbd/m.PB)
	}

	// Calculate charges
	m.qbs = cbs * m.vbs
	m.qbd = cbd * m.vbd
}

// UpdateVoltages from solution vector
func (m *Mosfet) UpdateVoltages(voltages []float64) error {
	nodeG := m.Nodes[1] // Gate
	nodeD := m.Nodes[0] // Drain
	nodeS := m.Nodes[2] // Source
	nodeB := m.Nodes[3] // Bulk

	vg := voltages[nodeG]
	vd := voltages[nodeD]
	vs := voltages[nodeS]
	vb := voltages[nodeB]

	// Type NMOS: +1, PMOS: -1
	typeValue := 1.0
	if m.Type == "PMOS" {
		typeValue = -1.0
	}

	m.vgs = typeValue * (vg - vs)
	m.vds = typeValue * (vd - vs)
	m.vbs = typeValue * (vb - vs)

	m.vgd = m.vgs - m.vds
	m.vbd = m.vbs - m.vds

	return nil
}

// Stamp method for matrix
func (m *Mosfet) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if status.Mode == ACAnalysis {
		return m.StampAC(matrix, status)
	}

	nd := m.Nodes[0] // Drain
	ng := m.Nodes[1] // Gate
	ns := m.Nodes[2] // Source
	nb := m.Nodes[3] // Bulk

	if m.vgs == 0 && m.vds == 0 && m.vbs == 0 {
		// Initial voltages for first iteration
		if m.Type == "NMOS" {
			m.vgs = 0.7 // Typical NMOS bias
			m.vds = 0.1 // Small drain bias
		} else {
			m.vgs = -0.7 // Typical PMOS bias
			m.vds = -0.1 // Small drain bias
		}
		m.vbs = 0.0
		m.vgd = m.vgs - m.vds
		m.vbd = m.vbs - m.vds
	}

	// Calculate currents and determine region
	m.id, m.region = m.calculateCurrents(m.vgs, m.vds, m.vbs, status.Temp)
	m.prevId = m.id

	m.calculateConductances()
	m.calculateCapacitances()

	gmin := status.Gmin

	if nd != 0 {
		// Drain
		matrix.AddElement(nd, nd, m.gds+gmin)
		if ng != 0 {
			matrix.AddElement(nd, ng, m.gm)
		}
		if ns != 0 {
			matrix.AddElement(nd, ns, -m.gds-m.gm-m.gmbs)
		}
		if nb != 0 {
			matrix.AddElement(nd, nb, m.gmbs)
		}
		matrix.AddRHS(nd, -m.id+m.gds*m.vds+m.gm*m.vgs+m.gmbs*m.vbs)
	}

	if ns != 0 {
		// Source
		matrix.AddElement(ns, ns, m.gds+m.gm+m.gmbs+gmin)
		if nd != 0 {
			matrix.AddElement(ns, nd, -m.gds)
		}
		if ng != 0 {
			matrix.AddElement(ns, ng, -m.gm)
		}
		if nb != 0 {
			matrix.AddElement(ns, nb, -m.gmbs)
		}
		matrix.AddRHS(ns, m.id-m.gds*m.vds-m.gm*m.vgs-m.gmbs*m.vbs)
	}

	// Gate and bulk
	if status.Mode == TransientAnalysis && status.TimeStep > 0 {
		dt := status.TimeStep

		m.calculateCharges()

		// Capacitive currents
		icgs := (m.qgs - m.prevQgs) / dt
		icgd := (m.qgd - m.prevQgd) / dt
		icgb := (m.qgb - m.prevQgb) / dt
		icbs := (m.qbs - m.prevQbs) / dt
		icbd := (m.qbd - m.prevQbd) / dt

		// Gate
		if ng != 0 {
			if nd != 0 {
				matrix.AddElement(ng, nd, m.cgd/dt)
				matrix.AddElement(nd, ng, m.cgd/dt)
				matrix.AddRHS(ng, icgd)
				matrix.AddRHS(nd, -icgd)
			}
			if ns != 0 {
				matrix.AddElement(ng, ns, m.cgs/dt)
				matrix.AddElement(ns, ng, m.cgs/dt)
				matrix.AddRHS(ng, icgs)
				matrix.AddRHS(ns, -icgs)
			}
			if nb != 0 {
				matrix.AddElement(ng, nb, m.cgb/dt)
				matrix.AddElement(nb, ng, m.cgb/dt)
				matrix.AddRHS(ng, icgb)
				matrix.AddRHS(nb, -icgb)
			}
			matrix.AddElement(ng, ng, (m.cgd+m.cgs+m.cgb)/dt)
		}

		// Bulk
		if nb != 0 {
			if ns != 0 {
				matrix.AddElement(nb, ns, m.CBS/dt)
				matrix.AddElement(ns, nb, m.CBS/dt)
				matrix.AddRHS(nb, icbs)
				matrix.AddRHS(ns, -icbs)
			}
			if nd != 0 {
				matrix.AddElement(nb, nd, m.CBD/dt)
				matrix.AddElement(nd, nb, m.CBD/dt)
				matrix.AddRHS(nb, icbd)
				matrix.AddRHS(nd, -icbd)
			}
			matrix.AddElement(nb, nb, (m.CBD+m.CBS)/dt)
		}
	}

	return nil
}

func (m *Mosfet) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	nd := m.Nodes[0] // Drain
	ng := m.Nodes[1] // Gate
	ns := m.Nodes[2] // Source
	nb := m.Nodes[3] // Bulk

	m.calculateCapacitances()

	omega := 2.0 * math.Pi * status.Frequency // Angular frequency

	// Real and imaginary parts for admittance elements
	gdsi := omega * 0.0   // No imaginary part for drain-source conductance
	gmi := omega * 0.0    // No imaginary part for transconductance
	gmbsi := omega * 0.0  // No imaginary part for body-effect transconductance
	cgsi := omega * m.cgs // Imaginary part for gate-source capacitance
	cgdi := omega * m.cgd // Imaginary part for gate-drain capacitance
	cgbi := omega * m.cgb // Imaginary part for gate-bulk capacitance
	cbsi := omega * m.CBS // Imaginary part for bulk-source capacitance
	cbdi := omega * m.CBD // Imaginary part for bulk-drain capacitance

	// Complex matrix
	if nd != 0 {
		// Drain
		matrix.AddComplexElement(nd, nd, m.gds, gdsi)
		if ng != 0 {
			matrix.AddComplexElement(nd, ng, m.gm, gmi+cgdi)
		}
		if ns != 0 {
			matrix.AddComplexElement(nd, ns, -m.gds-m.gm-m.gmbs, -gdsi-gmi-gmbsi)
		}
		if nb != 0 {
			matrix.AddComplexElement(nd, nb, m.gmbs, gmbsi+cbdi)
		}
	}

	if ns != 0 {
		// Source
		matrix.AddComplexElement(ns, ns, m.gds+m.gm+m.gmbs, gdsi+gmi+gmbsi)
		if nd != 0 {
			matrix.AddComplexElement(ns, nd, -m.gds, -gdsi)
		}
		if ng != 0 {
			matrix.AddComplexElement(ns, ng, -m.gm, -gmi+cgsi)
		}
		if nb != 0 {
			matrix.AddComplexElement(ns, nb, -m.gmbs, -gmbsi+cbsi)
		}
	}

	if ng != 0 {
		// Gate
		matrix.AddComplexElement(ng, ng, 0.0, cgsi+cgdi+cgbi)
		if nd != 0 {
			matrix.AddComplexElement(ng, nd, 0.0, cgdi)
		}
		if ns != 0 {
			matrix.AddComplexElement(ng, ns, 0.0, cgsi)
		}
		if nb != 0 {
			matrix.AddComplexElement(ng, nb, 0.0, cgbi)
		}
	}

	if nb != 0 {
		// Bulk
		matrix.AddComplexElement(nb, nb, 0.0, cbsi+cbdi+cgbi)
		if nd != 0 {
			matrix.AddComplexElement(nb, nd, 0.0, cbdi)
		}
		if ns != 0 {
			matrix.AddComplexElement(nb, ns, 0.0, cbsi)
		}
		if ng != 0 {
			matrix.AddComplexElement(nb, ng, 0.0, cgbi)
		}
	}

	return nil
}

func (m *Mosfet) LoadConductance(matrix matrix.DeviceMatrix) error {
	nd := m.Nodes[0] // Drain
	ng := m.Nodes[1] // Gate
	ns := m.Nodes[2] // Source
	nb := m.Nodes[3] // Bulk

	gmin := 1e-12

	if nd != 0 {
		matrix.AddElement(nd, nd, m.gds+gmin)
		if ng != 0 {
			matrix.AddElement(nd, ng, m.gm)
		}
		if ns != 0 {
			matrix.AddElement(nd, ns, -m.gds-m.gm-m.gmbs)
		}
		if nb != 0 {
			matrix.AddElement(nd, nb, m.gmbs)
		}
	}

	if ns != 0 {
		matrix.AddElement(ns, ns, m.gds+m.gm+m.gmbs+gmin)
		if nd != 0 {
			matrix.AddElement(ns, nd, -m.gds)
		}
		if ng != 0 {
			matrix.AddElement(ns, ng, -m.gm)
		}
		if nb != 0 {
			matrix.AddElement(ns, nb, -m.gmbs)
		}
	}

	return nil
}

func (m *Mosfet) LoadCurrent(matrix matrix.DeviceMatrix) error {
	nd := m.Nodes[0] // Drain
	ns := m.Nodes[2] // Source

	// Add MOSFET current contributions to RHS
	if nd != 0 {
		matrix.AddRHS(nd, -m.id+m.gds*m.vds+m.gm*m.vgs+m.gmbs*m.vbs)
	}

	if ns != 0 {
		matrix.AddRHS(ns, m.id-m.gds*m.vds-m.gm*m.vgs-m.gmbs*m.vbs)
	}

	return nil
}

func (m *Mosfet) UpdateState(voltages []float64, status *CircuitStatus) {
	// Charge
	m.prevQgs = m.qgs
	m.prevQgd = m.qgd
	m.prevQgb = m.qgb
	m.prevQbs = m.qbs
	m.prevQbd = m.qbd

	m.prevId = m.id // Current

	m.calculateCharges() // Update charges for next timestep
}

func (m *Mosfet) GetVgs() float64 {
	return m.vgs
}

func (m *Mosfet) GetVds() float64 {
	return m.vds
}

func (m *Mosfet) GetVbs() float64 {
	return m.vbs
}

func (m *Mosfet) GetId() float64 {
	return m.id
}

// transconductance
func (m *Mosfet) GetGm() float64 {
	return m.gm
}

// drain-source conductance
func (m *Mosfet) GetGds() float64 {
	return m.gds
}

// Operation region
func (m *Mosfet) GetRegion() int {
	return m.region
}

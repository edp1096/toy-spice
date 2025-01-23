package device

import (
	"fmt"
	"math"
	"toy-spice/internal/consts"
	"toy-spice/pkg/matrix"
)

// Bjt Gummel-Poon model
type Bjt struct {
	BaseDevice
	// DC Model Parameters
	Is  float64 // Transport saturation current
	Bf  float64 // Ideal maximum forward beta
	Br  float64 // Ideal maximum reverse beta
	Nf  float64 // Forward emission coefficient
	Nr  float64 // Reverse emission coefficient
	Vaf float64 // Forward Early voltage
	Var float64 // Reverse Early voltage
	Ikf float64 // Forward beta roll-off corner current
	Ikr float64 // Reverse beta roll-off corner current
	Ise float64 // B-E leakage saturation current
	Ne  float64 // B-E leakage emission coefficient
	Isc float64 // B-C leakage saturation current
	Nc  float64 // B-C leakage emission coefficient
	Rc  float64 // Collector resistance
	Re  float64 // Emitter resistance
	Rb  float64 // Zero bias base resistance
	Rbm float64 // Minimum base resistance
	Irb float64 // Current for base resistance=(rb+rbm)/2

	// AC & Capacitance Parameters
	Cje  float64 // B-E zero-bias depletion capacitance
	Vje  float64 // B-E built-in potential
	Mje  float64 // B-E grading coefficient
	Fc   float64 // Forward bias depletion capacitance coefficient
	Cjc  float64 // B-C zero-bias depletion capacitance
	Vjc  float64 // B-C built-in potential
	Mjc  float64 // B-C grading coefficient
	Xcjc float64 // Fraction of B-C depletion capacitance connected to internal base
	Tf   float64 // Ideal forward transit time
	Xtf  float64 // Transit time bias dependence coefficient
	Vtf  float64 // Transit time dependency on Vbc
	Itf  float64 // Transit time dependency on Ic
	Tr   float64 // Ideal reverse transit time

	// Temperature Parameters
	Xtb  float64 // Forward and reverse beta temperature coefficient
	Eg   float64 // Energy gap for temperature effect on Is
	Xti  float64 // Temperature exponent for effect on Is
	Tnom float64 // Parameter measurement temperature

	// Internal states
	vbe  float64 // B-E voltage
	vbc  float64 // B-C voltage
	vce  float64 // C-E voltage
	ic   float64 // Collector current
	ib   float64 // Base current
	ie   float64 // Emitter current
	gm   float64 // Transconductance
	gpi  float64 // Input conductance
	gmu  float64 // Feedback conductance
	gout float64 // Output conductance
	qbe  float64 // B-E charge storage
	qbc  float64 // B-C charge storage

	// Previous states for transient
	prevVbe float64
	prevVbc float64
	prevIc  float64
	prevIb  float64
	prevQbe float64
	prevQbc float64
}

func NewBJT(name string, nodeNames []string) *Bjt {
	if len(nodeNames) != 3 {
		panic(fmt.Sprintf("bjt %s: requires exactly 3 nodes", name))
	}
	b := &Bjt{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
	}
	b.setDefaultParameters()
	return b
}

func (b *Bjt) GetType() string { return "Q" }

func (b *Bjt) setDefaultParameters() {
	// DC parameters
	b.Is = 1e-16  // Transport saturation current
	b.Bf = 100.0  // Ideal maximum forward beta
	b.Br = 1.0    // Ideal maximum reverse beta
	b.Nf = 1.0    // Forward emission coefficient
	b.Nr = 1.0    // Reverse emission coefficient
	b.Vaf = 100.0 // Forward Early voltage
	b.Var = 100.0 // Reverse Early voltage
	b.Ikf = 0.01  // Forward beta roll-off corner current
	b.Ikr = 0.01  // Reverse beta roll-off corner current
	b.Ise = 0.0   // B-E leakage saturation current
	b.Ne = 1.5    // B-E leakage emission coefficient
	b.Isc = 0.0   // B-C leakage saturation current
	b.Nc = 2.0    // B-C leakage emission coefficient
	b.Rc = 0.0    // Collector resistance
	b.Re = 0.0    // Emitter resistance
	b.Rb = 0.0    // Zero bias base resistance
	b.Rbm = b.Rb  // Minimum base resistance
	b.Irb = 0.0   // Current for base resistance=(rb+rbm)/2

	// AC & Capacitance parameters
	b.Cje = 0.0  // B-E zero-bias depletion capacitance
	b.Vje = 0.75 // B-E built-in potential
	b.Mje = 0.33 // B-E junction exponential factor
	b.Fc = 0.5   // Forward bias depletion capacitance coefficient
	b.Cjc = 0.0  // B-C zero-bias depletion capacitance
	b.Vjc = 0.75 // B-C built-in potential
	b.Mjc = 0.33 // B-C junction exponential factor
	b.Xcjc = 1.0 // Fraction of B-C capacitance connected to internal base
	b.Tf = 0.0   // Ideal forward transit time
	b.Xtf = 0.0  // Transit time bias dependence coefficient
	b.Vtf = 0.0  // Transit time dependency on Vbc
	b.Itf = 0.0  // Transit time dependency on Ic
	b.Tr = 0.0   // Ideal reverse transit time

	// Temperature parameters
	b.Xtb = 0.0     // Forward and reverse beta temp. exp
	b.Eg = 1.11     // Energy gap
	b.Xti = 3.0     // Temp. exponent for Is
	b.Tnom = 300.15 // Nominal temperature (27°C)
}

func (b *Bjt) SetModelParameters(params map[string]float64) {
	paramsSet := map[string]*float64{
		// DC Parameters
		"is":  &b.Is,
		"bf":  &b.Bf,
		"br":  &b.Br,
		"nf":  &b.Nf,
		"nr":  &b.Nr,
		"vaf": &b.Vaf,
		"var": &b.Var,
		"ikf": &b.Ikf,
		"ikr": &b.Ikr,
		"ise": &b.Ise,
		"ne":  &b.Ne,
		"isc": &b.Isc,
		"nc":  &b.Nc,
		"rc":  &b.Rc,
		"re":  &b.Re,
		"rb":  &b.Rb,
		"rbm": &b.Rbm,
		"irb": &b.Irb,

		// AC & Capacitance Parameters
		"cje":  &b.Cje,
		"vje":  &b.Vje,
		"mje":  &b.Mje,
		"fc":   &b.Fc,
		"cjc":  &b.Cjc,
		"vjc":  &b.Vjc,
		"mjc":  &b.Mjc,
		"xcjc": &b.Xcjc,
		"tf":   &b.Tf,
		"xtf":  &b.Xtf,
		"vtf":  &b.Vtf,
		"itf":  &b.Itf,
		"tr":   &b.Tr,

		// Temperature Parameters
		"xtb":  &b.Xtb,
		"eg":   &b.Eg,
		"xti":  &b.Xti,
		"tnom": &b.Tnom,
	}

	for key, param := range paramsSet {
		if value, ok := params[key]; ok {
			*param = value
		}
	}
}

func (b *Bjt) thermalVoltage(temp float64) float64 {
	if temp <= 0 {
		temp = 300.15
	}
	return consts.BOLTZMANN * temp / consts.CHARGE
}

func (b *Bjt) temperatureAdjustedIs(temp float64) float64 {
	// vt := b.thermalVoltage(temp)
	ratio := temp / b.Tnom

	// Bandgap voltage difference
	// vt1 := b.thermalVoltage(b.Tnom)
	vg := b.Eg * consts.CHARGE // Convert Eg to joules
	dvg := vg * (1 - temp/b.Tnom)

	// Temperature adjustment
	arg := dvg/(consts.BOLTZMANN*temp) + b.Xti*math.Log(ratio)
	return b.Is * math.Pow(ratio, b.Xti/b.Nf) * math.Exp(arg)
}

func (b *Bjt) temperatureAdjustedBeta(temp float64) (float64, float64) {
	ratio := temp / b.Tnom

	// Beta temperature dependence
	bf := b.Bf * math.Pow(ratio, b.Xtb)
	br := b.Br * math.Pow(ratio, b.Xtb)

	return bf, br
}

func (b *Bjt) temperatureAdjustedLeakage(temp float64) (float64, float64) {
	vt := b.thermalVoltage(temp)
	ratio := temp / b.Tnom

	// B-E and B-C leakage currents temperature adjustment
	ise_t := b.Ise * math.Pow(ratio, b.Xti/b.Ne) * math.Exp(b.Eg/vt*(temp/b.Tnom-1.0))
	isc_t := b.Isc * math.Pow(ratio, b.Xti/b.Nc) * math.Exp(b.Eg/vt*(temp/b.Tnom-1.0))

	return ise_t, isc_t
}

func (b *Bjt) calculateCurrents(vbe, vbc, temp float64) (float64, float64, float64) {
	vt := b.thermalVoltage(temp)
	is_t := b.temperatureAdjustedIs(temp)
	bf_t, br_t := b.temperatureAdjustedBeta(temp)

	// Forward current (B-E diode)
	iF, _ := b.diodeCurrentSlope(vbe, is_t, vt)

	// Reverse current (B-C diode)
	iR, _ := b.diodeCurrentSlope(vbc, is_t, vt)

	// Early voltage effect
	qb := b.calculateChargeFactors(vbe, vbc, iF, iR)
	if b.Vaf > 0 {
		iF *= (1.0 + vbc/math.Max(b.Vaf, 1e-10))
	}
	if b.Var > 0 {
		iR *= (1.0 + vbe/math.Max(b.Var, 1e-10))
	}

	// High-level injection
	if b.Ikf > 0 {
		iF /= (1.0 + math.Abs(iF/(b.Ikf*qb)))
	}
	if b.Ikr > 0 {
		iR /= (1.0 + math.Abs(iR/(b.Ikr*qb)))
	}

	// Base current
	ib := iF/bf_t + iR/br_t

	// Collector current
	ic := iF - iR

	// Emitter current
	ie := -(ic + ib)

	return ic, ib, ie
}

func (b *Bjt) calculateChargeFactors(vbe, vbc, iF, iR float64) float64 {
	// Calculate base charge factor for high-level injection
	// q1 := 1.0 / (1.0 - vbc/b.Vaf - vbe/b.Var)
	q1 := 1.0
	if b.Vaf > 0 || b.Var > 0 {
		q1 = 1.0 / (1.0 - vbc/math.Max(b.Vaf, 1e-10) - vbe/math.Max(b.Var, 1e-10))
	}
	q2 := 0.0

	if b.Ikf > 0 {
		q2 += iF / b.Ikf
	}
	if b.Ikr > 0 {
		q2 += iR / b.Ikr
	}

	return q1 * (1.0 + (1.0+4.0*q2)*0.5)
}

func (b *Bjt) calculateConductances(vbe, vbc, ic, ib, temp float64) (float64, float64, float64, float64) {
	vt := b.thermalVoltage(temp)
	is_t := b.temperatureAdjustedIs(temp)

	// 최소값 보장
	const gmin = 1e-12

	// Transconductance
	gm := math.Max(math.Abs(ic)/(b.Nf*vt)+is_t/(b.Nf*vt), gmin)

	// Input conductance
	gpi := math.Max(math.Abs(ib)/(b.Nf*vt)+is_t/(b.Nf*vt), gmin)

	// Reverse transconductance
	gmu := math.Max(is_t/(b.Nr*vt), gmin)

	// Output conductance
	gout := gmin
	if b.Vaf > 0 {
		gout += math.Abs(ic) / math.Max(b.Vaf, 1.0)
	}

	return gm, gpi, gmu, gout
}

func (b *Bjt) calculateCapacitances(vbe, vbc float64) (float64, float64) {
	// B-E depletion capacitance
	cbe := b.Cje
	if b.Cje > 0 {
		if vbe < b.Fc*b.Vje {
			// Normal region
			arg := 1.0 - vbe/b.Vje
			if arg < 0.1 {
				arg = 0.1
			}
			cbe /= math.Pow(arg, b.Mje)
		} else {
			// Forward-bias region (above Fc*Vje)
			f1 := b.Vje * (1 - math.Pow(b.Fc, 1-b.Mje)) / (1 - b.Mje)
			f2 := math.Pow(b.Fc, -b.Mje)
			f3 := 1 - b.Fc*(1+b.Mje) + b.Mje*vbe/b.Vje
			cbe *= f2 * (1 + f3/f1)
		}
	}

	// B-C depletion capacitance
	cbc := b.Cjc
	if b.Cjc > 0 {
		if vbc < b.Fc*b.Vjc {
			// Normal region
			arg := 1.0 - vbc/b.Vjc
			if arg < 0.1 {
				arg = 0.1
			}
			cbc /= math.Pow(arg, b.Mjc)
		} else {
			// Forward-bias region (above Fc*Vjc)
			f1 := b.Vjc * (1 - math.Pow(b.Fc, 1-b.Mjc)) / (1 - b.Mjc)
			f2 := math.Pow(b.Fc, -b.Mjc)
			f3 := 1 - b.Fc*(1+b.Mjc) + b.Mjc*vbc/b.Vjc
			cbc *= f2 * (1 + f3/f1)
		}
	}

	return cbe, cbc
}

func (b *Bjt) calculateCharges(vbe, vbc, ic, temp float64) (float64, float64) {
	// Base charge due to diffusion (transit time)
	tf := b.Tf
	if b.Xtf > 0 {
		// Transit time bias dependence
		arg := 0.0
		if b.Vtf > 0 {
			arg = vbc / b.Vtf
			if arg > 0 {
				arg = 0
			}
		}
		tf *= (1 + b.Xtf*math.Exp(2*arg)*(ic/(ic+b.Itf)))
	}
	qd := tf * ic

	// Depletion charges
	cbe, cbc := b.calculateCapacitances(vbe, vbc)
	qbe := cbe * vbe
	qbc := cbc * vbc

	return qbe + qd, qbc
}

func (b *Bjt) calculateStorageTime(vbe, vbc, ic, temp float64) float64 {
	// Storage time calculation for transient analysis
	if ic > 0 {
		return b.Tf
	}
	return b.Tr
}

func (b *Bjt) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(b.Nodes) != 3 {
		return fmt.Errorf("bjt %s: requires exactly 3 nodes", b.Name)
	}

	// 1. Calculate operating point
	b.ic, b.ib, b.ie = b.calculateCurrents(b.vbe, b.vbc, status.Temp)
	b.gm, b.gpi, b.gmu, b.gout = b.calculateConductances(b.vbe, b.vbc, b.ic, b.ib, status.Temp)

	// 2. Add gmin
	gmin := status.Gmin
	b.gpi += gmin
	b.gmu += gmin
	b.gout += gmin

	// Get nodes
	nc := b.Nodes[0] // Collector
	nb := b.Nodes[1] // Base
	ne := b.Nodes[2] // Emitter

	// Debug prints
	fmt.Printf("BJT %s voltages: vbe=%.6f vbc=%.6f\n", b.Name, b.vbe, b.vbc)
	fmt.Printf("BJT %s currents: ic=%.6f ib=%.6f ie=%.6f\n", b.Name, b.ic, b.ib, b.ie)
	fmt.Printf("BJT %s conductances: gm=%.6f gpi=%.6f gmu=%.6f gout=%.6f\n", b.Name, b.gm, b.gpi, b.gmu, b.gout)
	fmt.Printf("BJT %s nodes: nc=%d nb=%d ne=%d\n", b.Name, nc, nb, ne)

	// 3. Stamp matrix
	if nc != 0 {
		// Collector node equations
		matrix.AddElement(nc, nc, b.gout+b.gmu)
		if nb != 0 {
			matrix.AddElement(nc, nb, -b.gmu+b.gm)
		}
		if ne != 0 {
			matrix.AddElement(nc, ne, -b.gout-b.gm)
		}
		// Collector current
		matrix.AddRHS(nc, -(b.ic - b.gout*b.vce + b.gmu*b.vbc - b.gm*b.vbe))
	}

	if nb != 0 {
		// Base node equations
		matrix.AddElement(nb, nb, b.gpi+b.gmu)
		if nc != 0 {
			matrix.AddElement(nb, nc, -b.gmu)
		}
		if ne != 0 {
			matrix.AddElement(nb, ne, -b.gpi)
		}
		// Base current
		matrix.AddRHS(nb, -(b.ib - b.gpi*b.vbe + b.gmu*b.vbc))
	}

	if ne != 0 {
		// Emitter node equations
		matrix.AddElement(ne, ne, b.gout+b.gpi+b.gm)
		if nc != 0 {
			matrix.AddElement(ne, nc, -b.gout-b.gm)
		}
		if nb != 0 {
			matrix.AddElement(ne, nb, -b.gpi)
		}
		// Emitter current
		matrix.AddRHS(ne, -b.ie)
	}

	return nil
}

func (b *Bjt) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(b.Nodes) != 3 {
		return fmt.Errorf("bjt %s: requires exactly 3 nodes", b.Name)
	}

	// Get nodes
	nc := b.Nodes[0] // Collector
	nb := b.Nodes[1] // Base
	ne := b.Nodes[2] // Emitter

	// Calculate small-signal conductances at operating point
	gm := b.gm     // Transconductance
	gpi := b.gpi   // Input conductance
	gmu := b.gmu   // Reverse transconductance
	gout := b.gout // Output conductance

	// Add capacitive effects
	omega := 2 * math.Pi * status.Frequency
	cbe, cbc := b.calculateCapacitances(b.vbe, b.vbc)

	// Add parasitic resistances if present
	if b.Rc > 0 || b.Re > 0 || b.Rb > 0 {
		if nc != 0 && b.Rc > 0 {
			matrix.AddComplexElement(nc, nc, 1.0/b.Rc, 0)
		}
		if ne != 0 && b.Re > 0 {
			matrix.AddComplexElement(ne, ne, 1.0/b.Re, 0)
		}
		if nb != 0 && b.Rb > 0 {
			matrix.AddComplexElement(nb, nb, 1.0/b.Rb, 0)
		}
	}

	// Stamp matrix for AC analysis
	if nc != 0 {
		// Collector node
		matrix.AddComplexElement(nc, nc, gout+gmu, omega*(cbc))
		if nb != 0 {
			matrix.AddComplexElement(nc, nb, -gmu+gm, -omega*(cbc))
		}
		if ne != 0 {
			matrix.AddComplexElement(nc, ne, -gout-gm, 0)
		}
	}

	if nb != 0 {
		// Base node
		matrix.AddComplexElement(nb, nb, gpi+gmu, omega*(cbe+cbc))
		if nc != 0 {
			matrix.AddComplexElement(nb, nc, -gmu, -omega*(cbc))
		}
		if ne != 0 {
			matrix.AddComplexElement(nb, ne, -gpi, -omega*(cbe))
		}
	}

	if ne != 0 {
		// Emitter node
		if nc != 0 {
			matrix.AddComplexElement(ne, nc, -gout-gm, 0)
		}
		if nb != 0 {
			matrix.AddComplexElement(ne, nb, -gpi, -omega*(cbe))
		}
		matrix.AddComplexElement(ne, ne, gout+gpi+gm, omega*(cbe))
	}

	return nil
}

func (b *Bjt) LoadConductance(matrix matrix.DeviceMatrix) error {
	nc := b.Nodes[0]
	nb := b.Nodes[1]
	ne := b.Nodes[2]

	// Load only conductance parts
	if nc != 0 {
		matrix.AddElement(nc, nc, b.gout+b.gmu)
		if nb != 0 {
			matrix.AddElement(nc, nb, -b.gmu+b.gm)
		}
		if ne != 0 {
			matrix.AddElement(nc, ne, -b.gout-b.gm)
		}
	}

	if nb != 0 {
		matrix.AddElement(nb, nb, b.gpi+b.gmu)
		if nc != 0 {
			matrix.AddElement(nb, nc, -b.gmu)
		}
		if ne != 0 {
			matrix.AddElement(nb, ne, -b.gpi)
		}
	}

	if ne != 0 {
		matrix.AddElement(ne, ne, b.gout+b.gpi+b.gm)
		if nc != 0 {
			matrix.AddElement(ne, nc, -b.gout-b.gm)
		}
		if nb != 0 {
			matrix.AddElement(ne, nb, -b.gpi)
		}
	}

	return nil
}

func (b *Bjt) LoadCurrent(matrix matrix.DeviceMatrix) error {
	nc := b.Nodes[0]
	nb := b.Nodes[1]
	ne := b.Nodes[2]

	// Load only current parts
	if nc != 0 {
		matrix.AddRHS(nc, -(b.ic - b.gout*b.vce + b.gmu*b.vbc - b.gm*b.vbe))
	}
	if nb != 0 {
		matrix.AddRHS(nb, -(b.ib - b.gpi*b.vbe + b.gmu*b.vbc))
	}
	if ne != 0 {
		matrix.AddRHS(ne, -(b.ie + b.gout*b.vce + b.gpi*b.vbe + b.gm*b.vbe))
	}

	return nil
}

func (b *Bjt) limitVbe(vbe float64) float64 {
	vt := b.thermalVoltage(300.15) // Room temperature
	if vbe > 0 {
		// Forward bias
		if vbe > 0.8 {
			// Severe forward bias - transition to linear
			vbe = 0.8 + vt*math.Log(1.0+(vbe-0.8)/(2.0*vt))
		}
	} else {
		// Reverse bias
		if vbe < -5.0 {
			vbe = -5.0
		}
	}
	return vbe
}

func (b *Bjt) limitVbc(vbc float64) float64 {
	vt := b.thermalVoltage(300.15)
	if vbc > 0 {
		// Forward bias (saturation)
		if vbc > 0.8 {
			vbc = 0.8 + vt*math.Log(1.0+(vbc-0.8)/(2.0*vt))
		}
	} else {
		// Reverse bias (normal operation)
		if vbc < -5.0 {
			vbc = -5.0
		}
	}
	return vbc
}

func (b *Bjt) limitExp(x float64) float64 {
	if x > 80.0 {
		return math.Exp(80.0)
	}
	if x < -80.0 {
		return math.Exp(-80.0)
	}
	return math.Exp(x)
}

func (b *Bjt) UpdateVoltages(voltages []float64) error {
	if len(b.Nodes) != 3 {
		return fmt.Errorf("bjt %s: requires exactly 3 nodes", b.Name)
	}

	// Get node voltages
	var vc, vb, ve float64
	if b.Nodes[0] != 0 { // Collector
		vc = voltages[b.Nodes[0]]
	}
	if b.Nodes[1] != 0 { // Base
		vb = voltages[b.Nodes[1]]
	}
	if b.Nodes[2] != 0 { // Emitter
		ve = voltages[b.Nodes[2]]
	}

	// 전압 제한
	vt := b.thermalVoltage(300.15)

	// B-E 접합
	vbe := vb - ve
	if vbe > 0.7 {
		vbe = 0.7 + vt*math.Log(1.0+(vbe-0.7)/(2.0*vt))
	} else if vbe < -0.6 {
		vbe = -0.6
	}

	// B-C 접합
	vbc := vb - vc
	if vbc > 0.7 {
		vbc = 0.7 + vt*math.Log(1.0+(vbc-0.7)/(2.0*vt))
	} else if vbc < -0.6 {
		vbc = -0.6
	}

	b.vbe = vbe
	b.vbc = vbc
	b.vce = vc - ve

	return nil
}

// Called during circuit setup
func (b *Bjt) InitializeOP() {
	// Typical initial bias for silicon BJT
	b.vbe = 0.7 // Forward active region
	b.vce = 1.0 // Prevent saturation
	b.vbc = b.vbe - b.vce

	// Initial currents
	vt := b.thermalVoltage(300.15)
	is_t := b.temperatureAdjustedIs(300.15)

	// Conservative initial guess
	b.ic = is_t * b.limitExp(b.vbe/vt)
	b.ib = b.ic / b.Bf
	b.ie = -(b.ic + b.ib)

	// Initial conductances
	b.gm = b.ic / vt
	b.gpi = b.ib / vt
	b.gmu = 1e-12
	b.gout = 1e-12
}

func (b *Bjt) UpdateState(voltages []float64, status *CircuitStatus) {
	// Store previous state for transient analysis
	b.prevVbe = b.vbe
	b.prevVbc = b.vbc
	b.prevIc = b.ic
	b.prevIb = b.ib
	b.prevQbe = b.qbe
	b.prevQbc = b.qbc
}

func (b *Bjt) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	// Local truncation error estimation for time step control
	dv := math.Max(math.Abs(b.vbe-b.prevVbe), math.Abs(b.vbc-b.prevVbc))
	di := math.Max(math.Abs(b.ic-b.prevIc), math.Abs(b.ib-b.prevIb))

	return math.Max(dv, di)
}

func (b *Bjt) SetTimeStep(dt float64) {
	// Nothing to do for BJT
}

func (b *Bjt) limitVoltage(v, vt float64) float64 {
	arg := v / vt
	if arg > 40.0 {
		return 40.0 * vt
	}
	if arg < math.Log(1e-40) {
		return math.Log(1e-40) * vt
	}
	return v
}

func (b *Bjt) diodeCurrentSlope(v, is, vt float64) (float64, float64) {
	// Returns current and conductance
	if v < -3.0*vt {
		return -is, 0.0
	}
	arg := b.limitVoltage(v, vt) / vt
	ev := b.limitExp(arg)
	current := is * (ev - 1.0)
	conductance := is * ev / vt
	return current, conductance
}

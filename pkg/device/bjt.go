package device

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/internal/consts"
	"github.com/edp1096/toy-spice/pkg/matrix"
)

// Node order: collector, base, emitter
type Bjt struct {
	BaseDevice
	NonLinear
	Type string

	// DC parameters
	Ies    float64 // Emitter saturation current (A)
	Ics    float64 // Collector saturation current (A)
	AlphaF float64 // Forward current gain (0.98~0.99)
	AlphaR float64 // Reverse current gain (0 ~ 0.5)
	Nf     float64 // Forward emission coefficient
	Nr     float64 // Reverse emission coefficient
	Ikf    float64 // forward roll-off corner current (A)
	Ikr    float64 // reverse roll-off corner current (A)
	Vaf    float64 // Forward Early voltage (V)
	Var    float64 // Reverse Early voltage (V)

	// Junction capacitances (depletion capacitance)
	Cje float64 // BE junction capacitance (F)
	Vje float64 // BE built-in potential (V)
	Mje float64 // BE grading coefficient

	Cjc float64 // BC junction capacitance (F)
	Vjc float64 // BC built-in potential (V)
	Mjc float64 // BC grading coefficient

	// Diffusion capacitance
	Tf float64 // transit time (s), BE diffusion capacitance = Tf * gm

	// Internal voltages (V)
	vbe float64 // Base-Emitter voltage
	vbc float64 // Base-Collector voltage
	vce float64 // Base-Collector voltage

	// DC current (A)
	ic float64 // Collector current
	ib float64 // Base current
	ie float64 // Emitter current

	// Conductance (S)
	gm   float64 // transconductance, dI_C/dV_BE
	gpi  float64 // Input/output conductance, I_B/V_T
	gout float64 // Output conductance

	Cbe float64 // BE capacitance (depletion+diffusion)
	Cbc float64 // BC capacitance

	// Charge (C)
	qbe float64 // BE charge
	qbc float64 // BC charge

	// Previous charge (C)
	prevQbe float64
	prevQbc float64
}

func NewBJT(name string, nodeNames []string) *Bjt {
	if len(nodeNames) != 3 {
		panic(fmt.Sprintf("Bjt %s: requires exactly 3 nodes (collector, base, emitter)", name))
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
	b.Ies = 1e-15
	b.Ics = 1e-15
	b.Nf = 1.0
	b.Nr = 1.0
	b.AlphaF = 0.98
	b.AlphaR = 0.5
	b.Ikf = 1e-3
	b.Ikr = 1e-3
	b.Vaf = 50.0
	b.Var = 50.0

	// Junction capacitances
	b.Cje = 1e-12 // BE capacitance (1pF)
	b.Vje = 0.7   // BE built-in potential (V)
	b.Mje = 0.33  // BE grading coefficient

	b.Cjc = 0.5e-12 // BC capacitance (0.5 pF)
	b.Vjc = 0.7     // BC built-in potential (V)
	b.Mjc = 0.33    // BC grading coefficient

	b.Tf = 300e-12 // 300 ps
}

func (b *Bjt) calculateInitialOperatingPoint(temp float64) {
	vt := b.thermalVoltage(temp)

	targetIc := 1e-3
	b.vbe = b.Nf * vt * math.Log(targetIc/b.Ies)
	b.vce = math.Max(2.0, b.vbe+1.0)

	b.vbc = b.vbe - b.vce

	fmt.Println("temp, vt, vbe, vce, vbc", temp, vt, b.vbe, b.vce, b.vbc)
}

func (b *Bjt) thermalVoltage(temp float64) float64 {
	if temp <= 0 {
		temp = 300.15
	}
	return consts.BOLTZMANN * temp / consts.CHARGE
}

func (b *Bjt) temperatureAdjustedIs(temp float64) float64 {
	tnom := 300.15
	ratio := temp / tnom
	eg := 1.11
	egFactor := (eg * consts.CHARGE / consts.BOLTZMANN) * (1/tnom - 1/temp)
	xti := 3.0

	return b.Ies * math.Pow(ratio, xti) * math.Exp(egFactor)
}

func (b *Bjt) SetModelParameters(params map[string]float64) {
	if typeVal, ok := params["type"]; ok {
		b.Type = "NPN"
		if typeVal == 1.0 {
			b.Type = "PNP"
		}
	}

	if val, ok := params["ies"]; ok {
		b.Ies = val
	}
	if val, ok := params["ics"]; ok {
		b.Ics = val
	}
	if val, ok := params["alphaf"]; ok {
		b.AlphaF = val
	}
	if val, ok := params["alphar"]; ok {
		b.AlphaR = val
	}
	if val, ok := params["ikf"]; ok {
		b.Ikf = val
	}
	if val, ok := params["ikr"]; ok {
		b.Ikr = val
	}
	if val, ok := params["vaf"]; ok {
		b.Vaf = val
	}
	if val, ok := params["var"]; ok {
		b.Var = val
	}
	// Junction capacitance
	if val, ok := params["cje"]; ok {
		b.Cje = val
	}
	if val, ok := params["vje"]; ok {
		b.Vje = val
	}
	if val, ok := params["mje"]; ok {
		b.Mje = val
	}
	if val, ok := params["cjc"]; ok {
		b.Cjc = val
	}
	if val, ok := params["vjc"]; ok {
		b.Vjc = val
	}
	if val, ok := params["mjc"]; ok {
		b.Mjc = val
	}
	if val, ok := params["tf"]; ok {
		b.Tf = val
	}
}

// Diffusion capacitance
func (b *Bjt) calculateCapacitances() {
	// BE junction: depletion capacitance
	if b.vbe < b.Vje {
		b.Cbe = b.Cje / math.Pow(1-b.vbe/b.Vje, b.Mje)
	} else {
		b.Cbe = b.Cje * (1 + b.Mje*(b.vbe-b.Vje)/b.Vje)
	}

	b.Cbe += b.Tf * b.gm // Append diffusion capacitance: Tf * gm

	// BC junction: depletion capacitance
	if b.vbc < b.Vjc {
		b.Cbc = b.Cjc / math.Pow(1-b.vbc/b.Vjc, b.Mjc)
	} else {
		b.Cbc = b.Cjc * (1 + b.Mjc*(b.vbc-b.Vjc)/b.Vjc)
	}
}

func (b *Bjt) calculateCurrents(temp float64) {
	vt := b.thermalVoltage(temp)
	expVbe := math.Exp(b.vbe / (b.Nf * vt))
	expVbc := math.Exp(b.vbc / (b.Nr * vt))

	sign := 1.0
	if b.Type == "PNP" {
		sign = -1.0
	}

	iF0 := sign * b.Ies * (expVbe - 1)
	iR0 := sign * b.Ics * (expVbc - 1)

	iF := iF0
	if b.Vaf > 0 {
		iF = iF0 * (1 - b.vbc/b.Vaf)
	}
	iR := iR0
	if b.Var > 0 {
		iR = iR0 * (1 + b.vbe/b.Var)
	}

	qb := 1.0
	if b.Vaf > 0 {
		qb = 1.0 / (1 - b.vbc/b.Vaf)
	}

	if b.Ikf > 0 {
		iF = iF / (1 + math.Abs(iF)/(b.Ikf*qb))
	}
	if b.Ikr > 0 {
		iR = iR / (1 + math.Abs(iR)/(b.Ikr*qb))
	}

	IE := sign * (iF - iR)
	IC := sign * ((b.AlphaF*iF - iR) / qb)
	IB := IE - IC

	b.ie = IE
	b.ic = IC
	b.ib = IB
}

func (b *Bjt) calculateConductances(temp float64) {
	vt := b.thermalVoltage(temp)
	expVbe := math.Exp(b.vbe / (b.Nf * vt))
	dIes_dVbe := b.Ies * expVbe / (b.Nf * vt)

	qb := 1.0
	if b.Vaf > 0 {
		qb = 1.0 / (1 - b.vbc/b.Vaf)
	}
	b.gm = b.AlphaF * dIes_dVbe / qb

	if vt != 0 {
		b.gpi = math.Abs(b.ib) / vt
	} else {
		b.gpi = 1e-12
	}

	if b.Vaf != 0 {
		b.gout = b.AlphaF * b.Ies * (expVbe - 1) * (1 / b.Vaf) * math.Pow(1+b.vce/b.Vaf, -2)
	} else {
		b.gout = 1e-12
	}

	fmt.Println("b.vbe, b.Nf, vt, expVbe, dIes_dVbe, gm, gpi, gout", b.vbe, b.Nf, vt, expVbe, dIes_dVbe, b.gm, b.gpi, b.gout)
}

func (b *Bjt) UpdateVoltages(voltages []float64) error {
	var vc, vb, ve float64
	if b.Nodes[0] != 0 {
		vc = voltages[b.Nodes[0]]
	}
	if b.Nodes[1] != 0 {
		vb = voltages[b.Nodes[1]]
	}
	if b.Nodes[2] != 0 {
		ve = voltages[b.Nodes[2]]
	}

	fmt.Printf("Node voltages: Vc=%.12f, Vb=%.12f, Ve=%.12f\n", vc, vb, ve)

	if b.Type == "PNP" {
		b.vbe = ve - vb
		b.vbc = vc - vb
		b.vce = ve - vc
	} else {
		b.vbe = vb - ve
		b.vbc = vb - vc
		b.vce = vc - ve
	}
	// b.vbe = ve - vb
	// b.vbc = vc - vb
	// b.vce = ve - vc

	fmt.Printf("Calculated voltages: Type: %s, VBE=%.12f, VBC=%.12f, VCE=%.12f\n", b.Type, b.vbe, b.vbc, b.vce)

	return nil
}

func (b *Bjt) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	nc := b.Nodes[0]
	nb := b.Nodes[1]
	ne := b.Nodes[2]

	// fmt.Printf("BJT %s type: %s\n", b.Name, b.Type)
	// fmt.Printf("Before calculation: VBE=%.3f, VCE=%.3f\n", b.vbe, b.vce)

	if b.vbe == 0 && b.vce == 0 {
		// // b.vbe = 0.7
		// // b.vce = 5.0
		// b.vbe = 0.685
		// b.vce = 2.7
		// b.vbc = b.vbe - b.vce

		b.calculateInitialOperatingPoint(status.Temp)
	}

	b.calculateCurrents(status.Temp)
	b.calculateConductances(status.Temp)
	b.calculateCapacitances()

	// fmt.Printf("After calculation: VBE=%.3f, VCE=%.3f\n", b.vbe, b.vce)

	// gmin := status.Gmin
	// b.gpi += gmin
	// b.gm += gmin
	// b.gout += gmin

	// Collector
	if nc != 0 {
		matrix.AddElement(nc, nc, b.gout)
		if nb != 0 {
			matrix.AddElement(nc, nb, -b.gout-b.gm)
		}
		if ne != 0 {
			matrix.AddElement(nc, ne, b.gm)
		}
		matrix.AddRHS(nc, -b.ic+b.gout*b.vce)
	}

	// Base
	if nb != 0 {
		matrix.AddElement(nb, nb, b.gpi)
		if nc != 0 {
			matrix.AddElement(nb, nc, -b.gpi)
		}
		matrix.AddRHS(nb, -b.ib+b.gpi*b.vbe)
	}

	// Emitter
	if ne != 0 {
		matrix.AddElement(ne, ne, b.gpi+b.gm)
		if nb != 0 {
			matrix.AddElement(ne, nb, -b.gpi-b.gm)
		}
		matrix.AddRHS(ne, -b.ie)
	}
	return nil
}

func (b *Bjt) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	nc := b.Nodes[0]
	nb := b.Nodes[1]
	ne := b.Nodes[2]

	b.calculateConductances(status.Temp)
	b.calculateCapacitances()

	omega := 2 * math.Pi * status.Frequency
	gmin := status.Gmin

	if nb != 0 {
		matrix.AddComplexElement(nb, nb, b.gpi+gmin, omega*b.Cbe)
		if nc != 0 {
			matrix.AddComplexElement(nb, nc, -b.gpi, 0)
		}
	}
	if nc != 0 {
		matrix.AddComplexElement(nc, nc, b.gout+gmin, 0)
		if nb != 0 {
			matrix.AddComplexElement(nc, nb, -b.gout-b.gm, 0)
		}
		if ne != 0 {
			matrix.AddComplexElement(nc, ne, b.gm, 0)
		}
	}
	if ne != 0 {
		matrix.AddComplexElement(ne, ne, b.gpi+b.gm+gmin, 0)
		if nb != 0 {
			matrix.AddComplexElement(ne, nb, -b.gpi-b.gm, 0)
		}
	}
	return nil
}

func (b *Bjt) StampTransient(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	dt := status.TimeStep
	dQbe := (b.qbe - b.prevQbe) / dt
	dQbc := (b.qbc - b.prevQbc) / dt

	nb := b.Nodes[1]
	if nb != 0 {
		matrix.AddRHS(nb, -dQbe-dQbc)
	}
	return nil
}

func (b *Bjt) LoadCurrent(matrix matrix.DeviceMatrix) error {
	nc := b.Nodes[0]
	nb := b.Nodes[1]
	ne := b.Nodes[2]

	if nc != 0 {
		matrix.AddRHS(nc, -b.ic+b.gout*b.vce)
	}
	if nb != 0 {
		matrix.AddRHS(nb, -b.ib+b.gpi*b.vbe)
	}
	if ne != 0 {
		matrix.AddRHS(ne, -b.ie)
	}

	return nil
}

func (b *Bjt) UpdateState(voltages []float64, status *CircuitStatus) {
	b.UpdateVoltages(voltages)
	b.prevQbe = b.qbe
	b.prevQbc = b.qbc

	b.calculateCurrents(status.Temp)
	b.calculateConductances(status.Temp)
	b.calculateCapacitances()
	b.qbe = b.Cbe * b.vbe
	b.qbc = b.Cbc * b.vbc
}

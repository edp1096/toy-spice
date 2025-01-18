package device

import (
	"fmt"
	"math"
	"toy-spice/pkg/matrix"
)

type Diode struct {
	BaseDevice
	// Model parameters
	Is   float64 // Saturation current 포화 전류
	N    float64 // Ideality Factor / Emission Coefficient 이상계수 / 발광계수
	Rs   float64 // Serial resistance
	Cj0  float64 // Zero-Bias junction capacitance
	M    float64 // Grading Coefficient 접합 기울기 계수
	Vj   float64 // Built-in Potential 접합 전위
	Bv   float64 // Breakdown voltage
	Gmin float64 // Minimum Conductance

	// Temperature parameters
	Eg  float64 // Energy Gap (eV)
	Xti float64 // Saturation current temperature exponent
	Tt  float64 // Transit time
	Fc  float64 // Forward-bias depletion capacitance coefficient

	// Internal states for Operating Point
	vd float64 // Voltage
	id float64 // Current
	gd float64 // Conductance at Operating Point

	// Status for Transient analysis
	vdOld      float64 // Previous voltage
	idOld      float64 // Previous current
	capCurrent float64 // Capacitive current
}

func NewDiode(name string, nodeNames []string) *Diode {
	if len(nodeNames) != 2 {
		panic(fmt.Sprintf("diode %s: requires exactly 2 nodes", name))
	}

	d := &Diode{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
	}
	d.setDefaultParameters()
	return d
}

func (d *Diode) GetType() string { return "D" }

func (d *Diode) setDefaultParameters() {
	d.Is = 1e-14   // 1e-14 A
	d.N = 1.0      // Ideality Factor / Emission Coefficient 이상계수 / 발광계수
	d.Rs = 0.0     // Serial resistance. not yet use
	d.Cj0 = 0.0    // Zero-Bias junction capacitance. not yet use
	d.M = 0.5      // Grading Coefficient 접합 기울기 계수
	d.Vj = 1.0     // Built-in Potential 접합 전위
	d.Bv = 100.0   // Breakdown voltage
	d.Gmin = 1e-12 // Minimum Conductance

	d.Eg = 1.11 // Silicon bandgap
	d.Xti = 3.0 // Saturation current temp. exp
	d.Tt = 0.0  // Transit time
	d.Fc = 0.5  // Forward-bias depletion capacitance coefficient
}

func (d *Diode) thermalVoltage(temp float64) float64 {
	const (
		CHARGE    = 1.6021918e-19 // Electron charge (C)
		BOLTZMANN = 1.3806226e-23 // Boltzmann constant (J/K)
	)

	if temp <= 0 {
		temp = 300.15
	}

	return BOLTZMANN * temp / CHARGE
}

func (d *Diode) SetModelParameters(params map[string]float64) {
	// Is (Saturation Current)
	if is, ok := params["is"]; ok {
		d.Is = is
	}

	// N (Emission Coefficient)
	if n, ok := params["n"]; ok {
		d.N = n
	}

	// Rs (Series Resistance)
	if rs, ok := params["rs"]; ok {
		d.Rs = rs
	}

	// Cj0 (Zero-bias junction capacitance)
	if cj0, ok := params["cj0"]; ok {
		d.Cj0 = cj0
	}

	// M (Grading coefficient)
	if m, ok := params["m"]; ok {
		d.M = m
	}

	// Vj (Junction potential)
	if vj, ok := params["vj"]; ok {
		d.Vj = vj
	}

	// Bv (Breakdown voltage)
	if bv, ok := params["bv"]; ok {
		d.Bv = bv
	}

	// Eg (Energy gap)
	if eg, ok := params["eg"]; ok {
		d.Eg = eg
	}

	// Xti (Saturation current temp. exp)
	if xti, ok := params["xti"]; ok {
		d.Xti = xti
	}

	// Tt (Transit time)
	if tt, ok := params["tt"]; ok {
		d.Tt = tt
	}

	// Fc (Forward-bias depletion capacitance coefficient)
	if fc, ok := params["fc"]; ok {
		d.Fc = fc
	}
}

func (d *Diode) temperatureAdjustedIs(temp float64) float64 {
	const REFTEMP = 300.15 // 27degC
	vt := d.thermalVoltage(temp)

	// is(T2) = is(T1) * (T2/T1)^(XTI/N) * exp(-(Eg/(2*k))*(1/T2 - 1/T1))
	ratio := temp / REFTEMP
	egfact := -d.Eg / (2 * vt) * (temp/REFTEMP - 1.0)

	return d.Is * math.Pow(ratio, d.Xti/d.N) * math.Exp(egfact)
}

func (d *Diode) calculateCurrent(vd, temp float64) float64 {
	vt := d.thermalVoltage(temp)
	nvt := d.N * vt

	// Forward bias and weak reverse bias
	if vd > -3.0*nvt {
		arg := vd / (nvt)
		if arg > 40.0 {
			arg = 40.0
		}
		evd := math.Exp(arg)
		is_t := d.temperatureAdjustedIs(temp)
		return is_t * (evd - 1.0)
	}

	return -d.temperatureAdjustedIs(temp)
}

func (d *Diode) calculateConductance(vd, id, temp float64) float64 {
	vt := d.thermalVoltage(temp)
	nvt := d.N * vt

	// Forward bias and weak reverse bias
	if vd > -3.0*nvt {
		return (math.Abs(id)+d.temperatureAdjustedIs(temp))/nvt + d.Gmin
	}

	// Strong reverse bias
	return d.Gmin
}

// Junction capacitance
func (d *Diode) calculateJunctionCap(vd float64) float64 {
	if d.Cj0 == 0 {
		return 0
	}

	if vd < 0 {
		arg := 1 - vd/d.Vj
		if arg < 0.1 {
			arg = 0.1
		}
		return d.Cj0 / math.Pow(arg, d.M)
	}

	// Forward bias
	return d.Cj0 * (1 + d.M*vd/d.Vj)
}

func (d *Diode) diffusionCapacitance(vd float64, temp float64, timeStep float64) float64 {
	if d.Tt == 0.0 || timeStep == 0.0 {
		return 0.0
	}

	// Current right now
	id := d.calculateCurrent(vd, temp)

	// dI/dt
	didt := (id - d.idOld) / timeStep

	// Transit Time capacitance: Cd = Tt * dI/dt
	return d.Tt * didt
}

// Stamp for OP/Transient
func (d *Diode) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if status.Mode == ACAnalysis {
		return d.StampAC(matrix, status)
	}

	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]

	// vt := d.thermalVoltage(status.Temp)
	// d.id = d.calculateCurrent(d.vd, vt)
	// d.gd = d.calculateConductance(d.vd, d.id, vt)
	d.id = d.calculateCurrent(d.vd, status.Temp)
	d.gd = d.calculateConductance(d.vd, d.id, status.Temp)

	// Diffusion capacitance
	if status.Mode == TransientAnalysis && status.TimeStep > 0 {
		cd := d.diffusionCapacitance(d.vd, status.Temp, status.TimeStep)
		// Add capacitive current to total current
		d.capCurrent = cd * (d.vd - d.vdOld) / status.TimeStep
		d.id += d.capCurrent
	}

	if n1 != 0 {
		matrix.AddElement(n1, n1, d.gd)
		if n2 != 0 {
			matrix.AddElement(n1, n2, -d.gd)
		}
		matrix.AddRHS(n1, -(d.id - d.gd*d.vd))
	}

	if n2 != 0 {
		if n1 != 0 {
			matrix.AddElement(n2, n1, -d.gd)
		}
		matrix.AddElement(n2, n2, d.gd)
		matrix.AddRHS(n2, (d.id - d.gd*d.vd))
	}

	return nil
}

// Stamp for AC
func (d *Diode) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	omega := 2 * math.Pi * status.Frequency

	// Conductance and capacitance at Operating Point
	gd := d.gd // Conductance
	cj := d.calculateJunctionCap(d.vd)

	// Admittance G + jωC
	yeq := complex(gd, omega*cj)

	if n1 != 0 {
		matrix.AddComplexElement(n1, n1, real(yeq), imag(yeq))
		if n2 != 0 {
			matrix.AddComplexElement(n1, n2, -real(yeq), -imag(yeq))
		}
	}

	if n2 != 0 {
		if n1 != 0 {
			matrix.AddComplexElement(n2, n1, -real(yeq), -imag(yeq))
		}
		matrix.AddComplexElement(n2, n2, real(yeq), imag(yeq))
	}

	return nil
}

func (d *Diode) LoadConductance(matrix matrix.DeviceMatrix) error {
	n1, n2 := d.Nodes[0], d.Nodes[1]

	if n1 != 0 {
		matrix.AddElement(n1, n1, d.gd)
		if n2 != 0 {
			matrix.AddElement(n1, n2, -d.gd)
		}
	}
	if n2 != 0 {
		if n1 != 0 {
			matrix.AddElement(n2, n1, -d.gd)
		}
		matrix.AddElement(n2, n2, d.gd)
	}

	return nil
}

func (d *Diode) LoadCurrent(matrix matrix.DeviceMatrix) error {
	n1, n2 := d.Nodes[0], d.Nodes[1]

	if n1 != 0 {
		matrix.AddRHS(n1, -(d.id - d.gd*d.vd))
	}
	if n2 != 0 {
		matrix.AddRHS(n2, (d.id - d.gd*d.vd))
	}

	return nil
}

func (d *Diode) SetTimeStep(dt float64) {}

func (d *Diode) UpdateState(voltages []float64, status *CircuitStatus) {
	d.vdOld = d.vd
	d.idOld = d.id - d.capCurrent // Store DC current only
	d.capCurrent = 0.0
}

func (d *Diode) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	return math.Abs(d.vd - d.vdOld)
}

func (d *Diode) UpdateVoltages(voltages []float64) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	var v1, v2 float64

	if n1 != 0 {
		v1 = voltages[n1]
	}
	if n2 != 0 {
		v2 = voltages[n2]
	}

	d.vd = v1 - v2
	return nil
}

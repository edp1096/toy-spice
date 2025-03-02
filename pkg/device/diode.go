package device

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/internal/consts"
	"github.com/edp1096/toy-spice/pkg/matrix"
)

type Diode struct {
	BaseDevice
	NonLinear

	// Model parameters
	Is   float64 // Saturation current
	N    float64 // Ideality Factor / Emission Coefficient
	Rs   float64 // Serial resistance
	Cj0  float64 // Zero-Bias junction capacitance
	M    float64 // Grading Coefficient
	Vj   float64 // Built-in Potential
	Bv   float64 // Breakdown voltage
	Gmin float64 // Minimum Conductance

	// Temperature parameters
	Eg  float64 // Energy Gap (eV)
	Xti float64 // Saturation current temperature exponent
	Tt  float64 // Transit time
	Fc  float64 // Forward-bias depletion capacitance coefficient

	// Internal states for Operating Point
	vd     float64 // Voltage
	id     float64 // Current
	charge float64 // charge
	gd     float64 // Conductance at Operating Point

	// Status for Transient analysis
	prevVd     float64 // Previous voltage
	prevId     float64 // Previous current
	prevCharge float64 // Previous charge
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
	d.N = 1.0      // Ideality Factor / Emission Coefficient
	d.Rs = 0.0     // Serial resistance. not yet use
	d.Cj0 = 0.0    // Zero-Bias junction capacitance. not yet use
	d.M = 0.5      // Grading Coefficient
	d.Vj = 1.0     // Built-in Potential
	d.Bv = 100.0   // Breakdown voltage
	d.Gmin = 1e-12 // Minimum Conductance

	d.Eg = 1.11 // Silicon bandgap
	d.Xti = 3.0 // Saturation current temp. exp
	d.Tt = 0.0  // Transit time
	d.Fc = 0.5  // Forward-bias depletion capacitance coefficient
}

func (d *Diode) thermalVoltage(temp float64) float64 {
	if temp <= 0 {
		temp = 300.15
	}

	return consts.BOLTZMANN * temp / consts.CHARGE
}

func (d *Diode) SetModelParameters(params map[string]float64) {
	paramsSet := map[string]*float64{
		"is":  &d.Is,  // Is (Saturation Current)
		"n":   &d.N,   // N (Emission Coefficient)
		"rs":  &d.Rs,  // Rs (Series Resistance)
		"cj0": &d.Cj0, // Cj0 (Zero-bias junction capacitance)
		"m":   &d.M,   // M (Grading coefficient)
		"vj":  &d.Vj,  // Vj (Junction potential)
		"bv":  &d.Bv,  // Bv (Breakdown voltage)
		"eg":  &d.Eg,  // Eg (Energy gap)
		"xti": &d.Xti, // Xti (Saturation current temp. exp)
		"tt":  &d.Tt,  // Tt (Transit time)
		"fc":  &d.Fc,  // Fc (Forward-bias depletion capacitance coefficient)
	}

	for key, param := range paramsSet {
		if value, ok := params[key]; ok {
			*param = value
		}
	}
}

func (d *Diode) temperatureAdjustedIs(temp float64) float64 {
	const ktemp = consts.KELVIN + 27 // 27degC
	vt := d.thermalVoltage(temp)

	// is(T2) = is(T1) * (T2/T1)^(XTI/N) * exp(-(Eg/(2*k))*(1/T2 - 1/T1))
	ratio := temp / ktemp
	egfact := -d.Eg / (2 * vt) * (temp/ktemp - 1.0)

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
	didt := (id - d.prevId) / timeStep

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

	d.id = d.calculateCurrent(d.vd, status.Temp)
	d.gd = d.calculateConductance(d.vd, d.id, status.Temp)

	if status.Mode == TransientAnalysis {
		d.charge = d.Tt * d.id

		if status.TimeStep > 0 {
			d.capCurrent = (d.charge - d.prevCharge) / status.TimeStep
			geq := d.Tt * d.gd / status.TimeStep

			d.gd += geq
			d.id += d.capCurrent
		}
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]

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
	d.prevVd = d.vd
	d.prevId = d.id - d.capCurrent // Store DC current only
	d.prevCharge = d.charge
	d.capCurrent = 0.0
}

func (d *Diode) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	return math.Abs(d.vd - d.prevVd)
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

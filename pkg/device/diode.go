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
}

func (d *Diode) thermalVoltage(temp float64) float64 {
	if temp <= 0 {
		temp = 300 // Default: room temperature. 300K
	}

	// VT = kT/q ≈ (0.026/300) * T
	return (0.026 / 300.0) * temp
}

// Bias current
func (d *Diode) calculateCurrent(vd float64, vt float64) float64 {
	// Forward bias
	if vd >= -5*vt {
		expArg := vd / (d.N * vt)
		if expArg > 40 { // Prevent overflow - exp(40) ≈ 10^17
			expArg = 40
		}
		expVt := math.Exp(expArg)

		return d.Is * (expVt - 1)
	}

	// Reverse breakdown
	if vd < -d.Bv {
		return -d.Is * (1 + (vd+d.Bv)/vt)
	}
	return -d.Is
}

// Conductance
func (d *Diode) calculateConductance(vd, id float64, vt float64) float64 {
	// Forward bias
	if vd >= -5*vt {
		return (id+d.Is)/(d.N*vt) + d.Gmin
	}

	// Reverse bias
	if vd < -d.Bv {
		return d.Is/vt + d.Gmin
	}

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

// Stamp for OP/Transient
func (d *Diode) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	vt := d.thermalVoltage(status.Temp)

	d.id = d.calculateCurrent(d.vd, vt)
	d.gd = d.calculateConductance(d.vd, d.id, vt)

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
	d.vdOld, d.idOld = d.vd, d.id
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

	// Node voltage
	if n1 != 0 {
		v1 = voltages[n1]
	}
	if n2 != 0 {
		v2 = voltages[n2]
	}

	// Diode voltage
	d.vd = v1 - v2
	return nil
}

package device

import (
	"math"
	"toy-spice/pkg/matrix"
	"toy-spice/pkg/util"
)

type Inductor struct {
	BaseDevice
	Current0  float64 // Current current
	Current1  float64 // Previous current
	Voltage0  float64 // Current voltage
	Voltage1  float64 // Previous voltage
	flux0     float64 // Current flux
	flux1     float64 // Previous flux
	branchIdx int     // Branch index
}

var _ TimeDependent = (*Inductor)(nil)

func NewInductor(name string, nodeNames []string, value float64) *Inductor {
	return &Inductor{
		BaseDevice: BaseDevice{
			Name:      name,
			Value:     value,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
	}
}

func (l *Inductor) GetType() string { return "L" }

func (l *Inductor) SetTimeStep(dt float64, status *CircuitStatus) { status.TimeStep = dt }

func (l *Inductor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := l.Nodes[0], l.Nodes[1]
	bIdx := l.branchIdx

	switch status.Mode {
	case ACAnalysis:
		omega := 2 * math.Pi * status.Frequency
		if n1 != 0 {
			matrix.AddComplexElement(n1, n1, 0, omega*l.Value)
			if n2 != 0 {
				matrix.AddComplexElement(n1, n2, 0, -omega*l.Value)
			}
		}
		if n2 != 0 {
			matrix.AddComplexElement(n2, n2, 0, omega*l.Value)
			if n1 != 0 {
				matrix.AddComplexElement(n2, n1, 0, -omega*l.Value)
			}
		}

	default:
		if n1 != 0 {
			matrix.AddElement(n1, bIdx, -1)
			matrix.AddElement(bIdx, n1, -1)
		}
		if n2 != 0 {
			matrix.AddElement(n2, bIdx, 1)
			matrix.AddElement(bIdx, n2, 1)
		}

		dt := status.TimeStep
		if dt <= 0 {
			dt = 1e-9
		}
		coeffs := util.GetIntegratorCoeffs(util.GearMethod, 1, dt)
		matrix.AddElement(bIdx, bIdx, -coeffs[0]*l.Value)

		matrix.AddRHS(bIdx, coeffs[0]*l.Value*l.Current1)
	}

	return nil
}

func (l *Inductor) LoadState(voltages []float64, status *CircuitStatus) {
	v1 := 0.0
	if l.Nodes[0] != 0 {
		v1 = voltages[l.Nodes[0]]
	}
	v2 := 0.0
	if l.Nodes[1] != 0 {
		v2 = voltages[l.Nodes[1]]
	}
	vd := v1 - v2
	dt := status.TimeStep

	l.Current0 = l.Current1 + (vd*dt)/l.Value
	l.flux0 = l.flux1 + vd*dt
}

func (l *Inductor) UpdateState(voltages []float64, status *CircuitStatus) {
	v1 := 0.0
	if l.Nodes[0] != 0 {
		v1 = voltages[l.Nodes[0]]
	}
	v2 := 0.0
	if l.Nodes[1] != 0 {
		v2 = voltages[l.Nodes[1]]
	}

	l.Voltage1 = l.Voltage0
	l.Voltage0 = v1 - v2

	l.Current1 = l.Current0

	equivR := l.Value / 1e-9
	l.Current0 = l.Voltage0 / equivR
}

func (l *Inductor) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	currentLTE := math.Abs(l.Current0-l.Current1) / (2.0 * status.TimeStep)
	voltageLTE := math.Abs(l.Voltage0-l.Voltage1) / (2.0 * status.TimeStep)

	return math.Max(currentLTE, voltageLTE)
}

func (l *Inductor) GetCurrent() float64 {
	return l.Current0
}

func (l *Inductor) GetPreviousCurrent() float64 {
	return l.Current1
}

func (l *Inductor) GetVoltage() float64 {
	return l.Voltage0
}

func (l *Inductor) GetPreviousVoltage() float64 {
	return l.Voltage1
}

// BranchIndex getter
func (l *Inductor) BranchIndex() int {
	return l.branchIdx
}

// BranchIndex setter
func (l *Inductor) SetBranchIndex(idx int) {
	l.branchIdx = idx
}

package device

import (
	"math"
	"toy-spice/pkg/matrix"
)

type Inductor struct {
	BaseDevice
	Current0 float64 // Current current
	Current1 float64 // Previous current
	Voltage0 float64 // Current voltage
	Voltage1 float64 // Previous voltage
	flux0    float64 // Current flux
	flux1    float64 // Previous flux
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

func (l *Inductor) SetTimeStep(dt float64) {}

func (l *Inductor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := l.Nodes[0], l.Nodes[1]

	switch status.Mode {
	case ACAnalysis:
		omega := 2 * math.Pi * status.Frequency
		indConductanceReal := 0.0
		indConductanceImag := -1.0 / (omega * l.Value) // -1/ωL

		if n1 != 0 {
			matrix.AddComplexElement(n1, n1, indConductanceReal, indConductanceImag)
			if n2 != 0 {
				matrix.AddComplexElement(n1, n2, -indConductanceReal, -indConductanceImag)
			}
		}
		if n2 != 0 {
			matrix.AddComplexElement(n2, n2, indConductanceReal, indConductanceImag)
			if n1 != 0 {
				matrix.AddComplexElement(n2, n1, -indConductanceReal, -indConductanceImag)
			}
		}

	case OperatingPointAnalysis:
		// OP
		gmin := 1e+12
		if n1 != 0 {
			matrix.AddElement(n1, n1, gmin)
		}
		if n2 != 0 {
			matrix.AddElement(n2, n2, gmin)
		}

	case TransientAnalysis:
		// Transient
		dt := status.TimeStep
		geq := dt / (2.0 * l.Value)
		ieq := l.Current1 + geq*(l.Voltage1-l.Voltage0)

		if n1 != 0 {
			matrix.AddElement(n1, n1, geq)
			matrix.AddRHS(n1, ieq)
		}
		if n2 != 0 {
			matrix.AddElement(n2, n2, geq)
			matrix.AddRHS(n2, -ieq)
		}
	}

	return nil
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
	vd := v1 - v2

	l.flux0 = l.flux1 + l.Voltage1*status.TimeStep
	l.Current0 = l.flux0 / l.Value

	l.Voltage1 = l.Voltage0
	l.Voltage0 = vd
	l.flux1 = l.flux0
}

func (l *Inductor) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	return math.Abs(l.Current0-l.Current1) / (2.0 * status.TimeStep)
}

func (l *Inductor) GetCurrent() float64 {
	return l.Current0
}

func (l *Inductor) GetVoltage() float64 {
	return l.Voltage0
}

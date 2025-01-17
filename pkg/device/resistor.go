package device

import (
	"fmt"
	"toy-spice/pkg/matrix"
)

type Resistor struct {
	BaseDevice
	Tc1  float64
	Tc2  float64
	Tnom float64
}

func NewResistor(name string, nodeNames []string, value float64) *Resistor {
	return &Resistor{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     value,
		},
		Tc1:  0.0,
		Tc2:  0.0,
		Tnom: 300.15,
	}
}

func (r *Resistor) GetType() string { return "R" }

func (r *Resistor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(r.Nodes) != 2 {
		return fmt.Errorf("resistor %s: requires exactly 2 nodes", r.Name)
	}

	n1, n2 := r.Nodes[0], r.Nodes[1]

	// g := 1.0 / r.Value // Conductance. G = 1/R
	g := 1.0 / r.temperatureAdjustedValue(status.Temp)

	switch status.Mode {
	case ACAnalysis:
		// AC
		if n1 != 0 {
			matrix.AddComplexElement(n1, n1, g, 0)
			if n2 != 0 {
				matrix.AddComplexElement(n1, n2, -g, 0)
			}
		}
		if n2 != 0 {
			if n1 != 0 {
				matrix.AddComplexElement(n2, n1, -g, 0)
			}
			matrix.AddComplexElement(n2, n2, g, 0)
		}

	default:
		// OP/Transient
		if n1 != 0 {
			matrix.AddElement(n1, n1, g)
			if n2 != 0 {
				matrix.AddElement(n1, n2, -g)
			}
		}
		if n2 != 0 {
			if n1 != 0 {
				matrix.AddElement(n2, n1, -g)
			}
			matrix.AddElement(n2, n2, g)
		}
	}

	return nil
}

func (r *Resistor) temperatureAdjustedValue(temp float64) float64 {
	dt := temp - r.Tnom
	factor := 1.0 + r.Tc1*dt + r.Tc2*dt*dt
	return r.Value * factor
}

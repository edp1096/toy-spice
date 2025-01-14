package device

import (
	"fmt"
	"toy-spice/pkg/matrix"
)

type Resistor struct {
	BaseDevice
}

func NewResistor(name string, nodeNames []string, value float64) *Resistor {
	return &Resistor{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     value,
		},
	}
}

func NewResistorNotUse(name string, nodeNames []string, value float64) *Resistor {
	return &Resistor{
		BaseDevice: *NewBaseDevice(name, value, nodeNames, "R"),
	}
}

func (r *Resistor) GetType() string { return "R" }

func (r *Resistor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(r.Nodes) != 2 {
		return fmt.Errorf("resistor %s: requires exactly 2 nodes", r.Name)
	}

	n1, n2 := r.Nodes[0], r.Nodes[1]
	g := 1.0 / r.Value // Conductance. G = 1/R

	switch status.Mode {
	case ACAnalysis:
		// Resistor for AC only has real conductance
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

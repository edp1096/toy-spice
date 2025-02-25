package device

import (
	"fmt"
	"math"
	"toy-spice/pkg/matrix"
)

type Mutual struct {
	BaseDevice
	inductors   []InductorComponent
	names       []string
	coefficient float64
}

func NewMutual(name string, indNames []string, k float64) *Mutual {
	return &Mutual{
		BaseDevice:  BaseDevice{Name: name},
		names:       indNames,
		coefficient: k,
		inductors:   make([]InductorComponent, len(indNames)),
	}
}

func (m *Mutual) GetType() string { return "K" }

func (m *Mutual) SetInductor(index int, ind InductorComponent) error {
	if index < 0 || index >= len(m.inductors) {
		return fmt.Errorf("invalid inductor index: %d", index)
	}
	m.inductors[index] = ind
	return nil
}

func (m *Mutual) GetInductor(index int) (InductorComponent, error) {
	if index < 0 || index >= len(m.inductors) {
		return nil, fmt.Errorf("invalid inductor index: %d", index)
	}
	return m.inductors[index], nil
}

func (m *Mutual) GetInductors() []InductorComponent {
	return m.inductors
}

func (m *Mutual) GetInductorNames() []string {
	return m.names
}

func (m *Mutual) GetNumInductors() int {
	return len(m.inductors)
}

func (m *Mutual) GetCoefficient() float64 { return m.coefficient }

func (m *Mutual) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(m.inductors) < 2 {
		return fmt.Errorf("mutual coupling %s requires at least two inductors", m.Name)
	}

	// Only for transient
	if status.Mode != TransientAnalysis {
		return nil
	}

	dt := status.TimeStep
	if dt <= 0 {
		return nil
	}

	var indInfo []struct {
		branchIdx int     // Branch index
		value     float64 // Inductance value
		current   float64 // Current for now
		nodes     [2]int  // Node indices
	}

	for _, ind := range m.inductors {
		branchIdx := 0
		if indObj, ok := ind.(*Inductor); ok {
			branchIdx = indObj.BranchIndex()
		} else {
			// MagneticInductor
			if magInd, ok := ind.(*MagneticInductor); ok {
				branchIdx = magInd.branchIdx
			} else {
				return fmt.Errorf("inductor %s does not support branch index", ind.GetName())
			}
		}

		indInfo = append(indInfo, struct {
			branchIdx int
			value     float64
			current   float64
			nodes     [2]int
		}{
			branchIdx: branchIdx,
			value:     ind.GetValue(),
			current:   ind.GetCurrent(),
			nodes:     [2]int{ind.GetNodes()[0], ind.GetNodes()[1]},
		})
	}

	// Mutual inductance processing for each inductor pair
	for i := range indInfo {
		for j := i + 1; j < len(indInfo); j++ {
			Mij := m.coefficient * math.Sqrt(indInfo[i].value*indInfo[j].value) // M = k * sqrt(L1 * L2)

			matrix.AddElement(indInfo[i].branchIdx, indInfo[j].branchIdx, -Mij/dt) // V1 = L1*di1/dt + M*di2/dt
			matrix.AddElement(indInfo[j].branchIdx, indInfo[i].branchIdx, -Mij/dt) // V2 = L2*di2/dt + M*di1/dt

			// RHS: Based on previous current
			matrix.AddRHS(indInfo[i].branchIdx, -Mij*indInfo[j].current/dt)
			matrix.AddRHS(indInfo[j].branchIdx, -Mij*indInfo[i].current/dt)
		}
	}

	return nil
}

func (m *Mutual) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(m.inductors) < 2 {
		return fmt.Errorf("mutual coupling %s requires at least two inductors", m.Name)
	}

	omega := 2 * math.Pi * status.Frequency
	n := len(m.inductors)

	// Get all inductors info
	L := make([]float64, n)
	nodes := make([][2]int, n)
	for i := range n {
		L[i] = m.inductors[i].GetValue()
		nodes[i] = [2]int{m.inductors[i].GetNodes()[0], m.inductors[i].GetNodes()[1]}
	}

	// AC Stamp
	for i := range n {
		for j := i + 1; j < n; j++ {
			Mij := m.coefficient * math.Sqrt(L[i]*L[j])

			if Mij != 0.0 {
				// AC admitance (jωM)
				yReal := 0.0
				yImag := omega * Mij

				if nodes[i][0] > 0 {
					if nodes[j][0] > 0 {
						matrix.AddComplexElement(nodes[i][0], nodes[j][0], yReal, yImag)
					}
					if nodes[j][1] > 0 {
						matrix.AddComplexElement(nodes[i][0], nodes[j][1], -yReal, -yImag)
					}
				}
				if nodes[i][1] > 0 {
					if nodes[j][0] > 0 {
						matrix.AddComplexElement(nodes[i][1], nodes[j][0], -yReal, -yImag)
					}
					if nodes[j][1] > 0 {
						matrix.AddComplexElement(nodes[i][1], nodes[j][1], yReal, yImag)
					}
				}
				if nodes[j][0] > 0 {
					if nodes[i][0] > 0 {
						matrix.AddComplexElement(nodes[j][0], nodes[i][0], yReal, yImag)
					}
					if nodes[i][1] > 0 {
						matrix.AddComplexElement(nodes[j][0], nodes[i][1], -yReal, -yImag)
					}
				}
				if nodes[j][1] > 0 {
					if nodes[i][0] > 0 {
						matrix.AddComplexElement(nodes[j][1], nodes[i][0], -yReal, -yImag)
					}
					if nodes[i][1] > 0 {
						matrix.AddComplexElement(nodes[j][1], nodes[i][1], yReal, yImag)
					}
				}
			}
		}
	}

	return nil
}

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

	dt := status.TimeStep
	n := len(m.inductors)

	if status.Mode == TransientAnalysis && dt > 0 {
		// 각 인덕터에 대한 정보 수집
		L := make([]float64, n)
		nodes := make([][2]int, n)
		for i := 0; i < n; i++ {
			L[i] = m.inductors[i].GetValue()
			nodes[i] = [2]int{m.inductors[i].GetNodes()[0], m.inductors[i].GetNodes()[1]}
		}

		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				// 상호 인덕턴스 계산
				Mij := m.coefficient * math.Sqrt(L[i]*L[j])

				if Mij != 0.0 {
					gij := dt / (2.0 * Mij)

					// ith iductor current/voltage
					vi := m.inductors[i].GetVoltage()
					// viPrev := m.inductors[i].GetPreviousVoltage()
					ii := m.inductors[i].GetCurrent()
					// iiPrev := m.inductors[i].GetPreviousCurrent()

					// jth inductor current/voltage
					vj := m.inductors[j].GetVoltage()
					// vjPrev := m.inductors[j].GetPreviousVoltage()
					ij := m.inductors[j].GetCurrent()
					// ijPrev := m.inductors[j].GetPreviousCurrent()

					// Equivalent current source
					ieqi := ii + gij*vj // i->j coupling
					ieqj := ij + gij*vi // j->i coupling
					// ieqi := ii + gij*(vj+vjPrev)/2.0
					// ieqj := ij + gij*(vi+viPrev)/2.0
					// ieqi := ii + gij*(ij-ijPrev)
					// ieqj := ij + gij*(ii-iiPrev)

					// 매트릭스 스탬핑
					if nodes[i][0] > 0 {
						if nodes[j][0] > 0 {
							matrix.AddElement(nodes[i][0], nodes[j][0], gij)
						}
						if nodes[j][1] > 0 {
							matrix.AddElement(nodes[i][0], nodes[j][1], -gij)
						}
						matrix.AddRHS(nodes[i][0], ieqi)
					}
					if nodes[i][1] > 0 {
						if nodes[j][0] > 0 {
							matrix.AddElement(nodes[i][1], nodes[j][0], -gij)
						}
						if nodes[j][1] > 0 {
							matrix.AddElement(nodes[i][1], nodes[j][1], gij)
						}
						matrix.AddRHS(nodes[i][1], -ieqi)
					}
					if nodes[j][0] > 0 {
						if nodes[i][0] > 0 {
							matrix.AddElement(nodes[j][0], nodes[i][0], gij)
						}
						if nodes[i][1] > 0 {
							matrix.AddElement(nodes[j][0], nodes[i][1], -gij)
						}
						matrix.AddRHS(nodes[j][0], ieqj)
					}
					if nodes[j][1] > 0 {
						if nodes[i][0] > 0 {
							matrix.AddElement(nodes[j][1], nodes[i][0], -gij)
						}
						if nodes[i][1] > 0 {
							matrix.AddElement(nodes[j][1], nodes[i][1], gij)
						}
						matrix.AddRHS(nodes[j][1], -ieqj)
					}
				}
			}
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

	// 각 인덕터에 대한 정보 수집
	L := make([]float64, n)
	nodes := make([][2]int, n)
	for i := 0; i < n; i++ {
		L[i] = m.inductors[i].GetValue()
		nodes[i] = [2]int{m.inductors[i].GetNodes()[0], m.inductors[i].GetNodes()[1]}
	}

	// SPICE3F5 스타일의 AC 스탬핑
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// SPICE3F5 방식의 상호 인덕턴스 계산
			Mij := m.coefficient * math.Sqrt(L[i]*L[j])

			if Mij != 0.0 {
				// AC 어드미턴스 (jωM)
				yReal := 0.0
				yImag := omega * Mij

				// 매트릭스 스탬핑
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

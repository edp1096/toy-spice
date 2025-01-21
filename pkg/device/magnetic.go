package device

import (
	"fmt"
	"math"
	"toy-spice/pkg/matrix"
)

// Constants
const mu0 = 4 * math.Pi * 1e-7 // 진공 투자율 (H/m)

// Magnetic component interface
type MagneticComponent interface {
	Device
	GetFlux() float64
	SetFlux(flux float64)
	GetCurrent() float64
	GetCore() *JilesAthertonCore
	SetCore(params map[string]float64)
}

// 변압기 인스턴스들을 관리하는 타입
type MagneticCore struct {
	JilesAthertonCore
	inductors []*MagneticInductor // 코어를 공유하는 인덕터들
}

// MagneticInductor 수정
type MagneticInductor struct {
	BaseDevice
	core     *MagneticCore // 기존의 JilesAthertonCore 대신
	turns    int
	current0 float64
	current1 float64
	flux0    float64
	flux1    float64
	voltage0 float64
	voltage1 float64
}

// Jiles-Atherton model parameters
type JilesAthertonCore struct {
	// Core parameters
	Ms    float64 // Saturation magnetization (A/m)
	alpha float64 // Domain coupling parameter
	a     float64 // Shape parameter
	c     float64 // Reversibility
	k     float64 // Pinning coefficient
	area  float64 // Cross-sectional area (m^2)
	len   float64 // Mean path length (m)
	tc    float64 // Curie temperature (K)
	beta  float64 // Temperature coefficient

	// State variables
	H    float64 // Applied field (A/m)
	Hold float64 // Previous field
	M    float64 // Total magnetization (A/m)
	Man  float64 // Anhysteretic magnetization
	Mirr float64 // Irreversible magnetization
	dMdH float64 // Differential permeability
	temp float64 // Operating temperature
}

func NewMagneticCore() *MagneticCore {
	return &MagneticCore{
		JilesAthertonCore: *NewJilesAthertonCore(),
		inductors:         make([]*MagneticInductor, 0),
	}
}

func (mc *MagneticCore) AddInductor(ind *MagneticInductor) {
	mc.inductors = append(mc.inductors, ind)
}

func NewJilesAthertonCore() *JilesAthertonCore {
	return &JilesAthertonCore{
		Ms:    1.6e6, // Default values
		alpha: 1e-3,
		a:     1000.0,
		c:     0.1,
		k:     2000.0,
		tc:    1043.0, // Iron Curie temperature
		beta:  0.0,
		area:  1e-4, // 1 cm^2
		len:   0.1,  // 10 cm
	}
}

func (c *JilesAthertonCore) Calculate(h float64, temp float64) (float64, float64) {
	c.temp = temp
	dH := h - c.Hold

	if math.Abs(dH) < 1e-12 {
		return c.M, c.dMdH
	}

	// Temperature scaling
	mst := c.Ms * math.Pow((c.tc-temp)/c.tc, c.beta)

	// Effective field including domain coupling
	he := h + c.alpha*c.M

	// Anhysteretic magnetization using modified Langevin function
	lan := func(x float64) float64 {
		if math.Abs(x) < 1e-6 {
			return x / 3.0
		}
		return 1.0/math.Tanh(x) - 1.0/x
	}

	c.Man = mst * lan(he/c.a)

	// Direction of magnetization change
	delta := 1.0
	if dH < 0 {
		delta = -1.0
	}

	// Irreversible magnetization change
	denom := c.k*delta - c.alpha*(c.Man-c.Mirr)
	if math.Abs(denom) < 1e-12 {
		denom = 1e-12 * math.Copysign(1.0, denom)
	}

	dMirrdH := (c.Man - c.Mirr) / denom
	c.Mirr += dMirrdH * dH

	// Total magnetization
	mold := c.M
	c.M = c.Mirr + c.c*(c.Man-c.Mirr)

	// Differential permeability
	c.dMdH = (c.M - mold) / dH
	if math.IsNaN(c.dMdH) || math.IsInf(c.dMdH, 0) {
		c.dMdH = mst / c.a / 3.0 // Initial permeability
	}

	c.H = h
	c.Hold = h

	return c.M, c.dMdH
}

func NewMagneticInductor(name string, nodeNames []string, turns int) *MagneticInductor {
	return &MagneticInductor{
		BaseDevice: BaseDevice{
			Name:      name,
			NodeNames: nodeNames,
			Nodes:     make([]int, len(nodeNames)),
		},
		turns: turns,
	}
}

func (m *MagneticInductor) GetType() string { return "L" }

func (m *MagneticInductor) GetValue() float64 {
	if m.core == nil {
		return 0
	}
	// 코어의 비선형성을 고려한 실효 인덕턴스
	_, dMdH := m.core.Calculate(float64(m.turns)*m.current0/m.core.len, 300.15)
	return mu0 * float64(m.turns*m.turns) * m.core.area * (1 + dMdH) / m.core.len
}

func (m *MagneticInductor) GetCurrent() float64 {
	return m.current0
}

func (m *MagneticInductor) GetVoltage() float64 {
	return m.voltage0
}

func (m *MagneticInductor) SetCore(params map[string]float64) {
	core := NewMagneticCore()
	// JilesAthertonCore 파라미터 설정
	if ms, ok := params["ms"]; ok {
		core.Ms = ms
	}
	if alpha, ok := params["alpha"]; ok {
		core.alpha = alpha
	}
	if a, ok := params["a"]; ok {
		core.a = a
	}
	if c, ok := params["c"]; ok {
		core.c = c
	}
	if k, ok := params["k"]; ok {
		core.k = k
	}
	if area, ok := params["area"]; ok {
		core.area = area
	}
	if length, ok := params["len"]; ok {
		core.len = length
	}

	m.core = core
	// 이 인덕터를 코어의 인덕터 리스트에 추가
	core.AddInductor(m)
}

func (m *MagneticInductor) GetCore() *MagneticCore {
	return m.core
}

func (m *MagneticInductor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if m.core == nil {
		return fmt.Errorf("magnetic core not set for inductor %s", m.Name)
	}

	n1, n2 := m.Nodes[0], m.Nodes[1]

	switch status.Mode {
	case OperatingPointAnalysis:
		// DC에서의 최소 컨덕턴스
		geq := 1e-9
		if n1 != 0 {
			matrix.AddElement(n1, n1, geq)
			if n2 != 0 {
				matrix.AddElement(n1, n2, -geq)
			}
		}
		if n2 != 0 {
			if n1 != 0 {
				matrix.AddElement(n2, n1, -geq)
			}
			matrix.AddElement(n2, n2, geq)
		}

	case TransientAnalysis:
		dt := status.TimeStep
		if dt > 0 {
			// 자기장 계산
			h := float64(m.turns) * m.current0 / m.core.len
			_, dMdH := m.core.Calculate(h, status.Temp)

			// 실효 인덕턴스 계산
			Leff := mu0 * float64(m.turns*m.turns) *
				m.core.area * (1 + dMdH) / m.core.len

			// 등가 컨덕턴스
			geq := dt / (2.0 * Leff)

			// 등가 전류원
			ieq := m.current1 + geq*(m.voltage1-m.voltage0)

			// 매트릭스 스탬핑
			if n1 != 0 {
				matrix.AddElement(n1, n1, geq)
				if n2 != 0 {
					matrix.AddElement(n1, n2, -geq)
				}
				matrix.AddRHS(n1, ieq)
			}
			if n2 != 0 {
				if n1 != 0 {
					matrix.AddElement(n2, n1, -geq)
				}
				matrix.AddElement(n2, n2, geq)
				matrix.AddRHS(n2, -ieq)
			}
		}
	}

	return nil
}

func (m *MagneticInductor) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if m.core == nil {
		return fmt.Errorf("magnetic core not set for inductor %s", m.Name)
	}

	n1, n2 := m.Nodes[0], m.Nodes[1]
	omega := 2 * math.Pi * status.Frequency

	// AC 해석에서는 동작점에서의 선형화된 인덕턴스 사용
	h := float64(m.turns) * m.current0 / m.core.len
	_, dMdH := m.core.Calculate(h, status.Temp)
	Leff := mu0 * float64(m.turns) * float64(m.turns) *
		m.core.area * (1 + dMdH) / m.core.len

	// Complex admittance
	yeqReal := 0.0
	yeqImag := -1.0 / (omega * Leff)

	if n1 != 0 {
		matrix.AddComplexElement(n1, n1, yeqReal, yeqImag)
		if n2 != 0 {
			matrix.AddComplexElement(n1, n2, -yeqReal, -yeqImag)
		}
	}
	if n2 != 0 {
		if n1 != 0 {
			matrix.AddComplexElement(n2, n1, -yeqReal, -yeqImag)
		}
		matrix.AddComplexElement(n2, n2, yeqReal, yeqImag)
	}

	return nil
}

func (m *MagneticInductor) UpdateState(voltages []float64, status *CircuitStatus) {
	// Update voltages
	v1 := 0.0
	if m.Nodes[0] != 0 {
		v1 = voltages[m.Nodes[0]]
	}
	v2 := 0.0
	if m.Nodes[1] != 0 {
		v2 = voltages[m.Nodes[1]]
	}

	m.voltage1 = m.voltage0
	m.voltage0 = v1 - v2

	// Update currents and flux
	m.current1 = m.current0
	m.flux1 = m.flux0

	dt := status.TimeStep
	if dt > 0 {
		dflux := m.voltage0 * dt
		m.flux0 += dflux

		// Update current from core model
		h := float64(m.turns) * m.current0 / m.core.len
		B, _ := m.core.Calculate(h, status.Temp)
		m.current0 = B * m.core.area * float64(m.turns) / m.flux0
	}
}

func (m *MagneticInductor) GetFlux() float64 {
	return m.flux0
}

func (m *MagneticInductor) SetFlux(flux float64) {
	m.flux0 = flux
}

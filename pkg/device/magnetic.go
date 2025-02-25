package device

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/matrix"
	"github.com/edp1096/toy-spice/pkg/util"
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
	core      *MagneticCore
	turns     int
	current0  float64
	current1  float64
	flux0     float64
	flux1     float64
	voltage0  float64
	voltage1  float64
	branchIdx int
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

	// Keep previous value if too low change
	if math.Abs(dH) < 1e-12 {
		return c.M, c.dMdH
	}

	// Magnetize direction
	delta := 1.0
	if dH < 0 {
		delta = -1.0
	}

	// 온도 스케일링
	mst := c.Ms
	if c.tc > 0 {
		mst *= math.Pow((c.tc-temp)/c.tc, c.beta)
	}

	// 유효 자기장
	he := h + c.alpha*c.M

	var Man float64
	if math.Abs(he) < 1e-6 {
		Man = mst * he / (3.0 * c.a)
	} else {
		Man = mst * (1.0/math.Tanh(he/c.a) - c.a/he)
	}

	denom := c.k*delta - c.alpha*(Man-c.Mirr)
	if math.Abs(denom) < 1e-12 {
		denom = 1e-12 * math.Copysign(1.0, denom)
	}
	dMirr_dH := (Man - c.Mirr) / denom

	c.Mirr += dMirr_dH * dH

	Mold := c.M

	c.M = c.Mirr + c.c*(Man-c.Mirr)
	c.dMdH = (c.M - Mold) / dH

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
	bIdx := m.branchIdx

	switch status.Mode {
	case OperatingPointAnalysis:
		if n1 != 0 {
			matrix.AddElement(n1, bIdx, -1)
			matrix.AddElement(bIdx, n1, -1)
		}
		if n2 != 0 {
			matrix.AddElement(n2, bIdx, 1)
			matrix.AddElement(bIdx, n2, 1)
		}

		var smallL float64 = 1e-3
		matrix.AddElement(bIdx, bIdx, smallL)

		m.current0 = 0
		m.current1 = 0
		m.flux0 = 0
		m.flux1 = 0

	case TransientAnalysis:
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

		if status.Time < dt || math.Abs(m.current0) < 1e-9 {
			mu0 := 4.0e-7 * math.Pi // 진공 투자율
			L0 := mu0 * float64(m.turns*m.turns) * m.core.area / m.core.len

			// v = L*di/dt => L/dt*i_now - L/dt*i_prev = v
			coeffs := util.GetIntegratorCoeffs(util.GearMethod, 1, dt)
			diag := coeffs[0] * L0

			matrix.AddElement(bIdx, bIdx, -diag)
			matrix.AddRHS(bIdx, diag*m.current1)

			return nil
		}

		h := float64(m.turns) * m.current0 / m.core.len
		h = math.Max(-1e6, math.Min(1e6, h))

		_, dMdH := m.core.Calculate(h, status.Temp) // dM/dH
		dMdH = math.Max(-1e3, math.Min(1e3, dMdH))  // dM/dH limit

		mu0 := 4.0e-7 * math.Pi
		muEff := mu0 * (1.0 + dMdH)
		Leff := muEff * float64(m.turns*m.turns) * m.core.area / m.core.len

		Leff = math.Max(1e-12, Leff)

		coeffs := util.GetIntegratorCoeffs(util.GearMethod, 1, dt)
		diag := coeffs[0] * Leff

		matrix.AddElement(bIdx, bIdx, -diag)
		rhs := diag * m.current1
		matrix.AddRHS(bIdx, rhs)
	}

	return nil
}

func (m *MagneticInductor) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if m.core == nil {
		return fmt.Errorf("magnetic core not set for inductor %s", m.Name)
	}

	n1, n2 := m.Nodes[0], m.Nodes[1]
	omega := 2 * math.Pi * status.Frequency

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

func (m *MagneticInductor) UpdateState(solution []float64, status *CircuitStatus) {
	m.voltage1 = m.voltage0
	m.current1 = m.current0
	m.flux1 = m.flux0

	m.voltage0 = 0
	if m.Nodes[0] > 0 {
		m.voltage0 += solution[m.Nodes[0]]
	}
	if m.Nodes[1] > 0 {
		m.voltage0 -= solution[m.Nodes[1]]
	}

	// Check for branch index
	if m.branchIdx >= len(solution) || m.branchIdx < 0 {
		return
	}

	m.current0 = -solution[m.branchIdx]

	dt := status.TimeStep
	if dt > 0 {
		m.flux0 = m.flux1 + m.voltage0*dt
	}
}

func (m *MagneticInductor) GetFlux() float64 {
	return m.flux0
}

func (m *MagneticInductor) SetFlux(flux float64) {
	m.flux0 = flux
}

func (m *MagneticInductor) BranchIndex() int {
	return m.branchIdx
}

func (m *MagneticInductor) SetBranchIndex(idx int) {
	m.branchIdx = idx
}

func (m *MagneticInductor) GetPreviousCurrent() float64 {
	return m.current1
}

func (m *MagneticInductor) GetPreviousVoltage() float64 {
	return m.voltage1
}

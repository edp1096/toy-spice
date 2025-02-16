package device

import (
	"math"
	"toy-spice/pkg/matrix"
)

type Inductor struct {
	BaseDevice
	Current0    float64   // 현재 전류
	Current1    float64   // 이전 전류
	Voltage0    float64   // 현재 전압
	Voltage1    float64   // 이전 전압
	branchIdx   int       // Branch 인덱스
	dState      []float64 // 적분 상태 저장 (최대 7개: BE + TR용 2개, Gear용 5개)
	prevDelta   float64   // 이전 시간 스텝
	orderState  int       // 현재 차수 상태
	maxOrder    int       // 최대 허용 차수
	methodState int       // 현재 적분 방법
}

func NewInductor(name string, nodeNames []string, value float64) *Inductor {
	return &Inductor{
		BaseDevice: BaseDevice{
			Name:      name,
			Value:     value,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
		dState:      make([]float64, 7),
		orderState:  1,
		maxOrder:    2, // TR 최대
		methodState: BE,
	}
}

func (l *Inductor) GetType() string { return "L" }

func (l *Inductor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := l.Nodes[0], l.Nodes[1]
	bIdx := l.branchIdx

	switch status.Mode {
	case OperatingPointAnalysis:
		if n1 != 0 {
			matrix.AddElement(n1, bIdx, 1)
			matrix.AddElement(bIdx, n1, 1)
		}
		if n2 != 0 {
			matrix.AddElement(n2, bIdx, -1)
			matrix.AddElement(bIdx, n2, -1)
		}
		matrix.AddElement(bIdx, bIdx, 1e3)

	case TransientAnalysis:
		if n1 != 0 {
			matrix.AddElement(n1, bIdx, 1)
			matrix.AddElement(bIdx, n1, 1)
		}
		if n2 != 0 {
			matrix.AddElement(n2, bIdx, -1)
			matrix.AddElement(bIdx, n2, -1)
		}
		matrix.AddElement(bIdx, bIdx, l.Value/status.TimeStep)
		matrix.AddRHS(bIdx, (l.Value/status.TimeStep)*l.Current1)
	}
	return nil
}

func (l *Inductor) LoadState(voltages []float64, status *CircuitStatus) {
	v1, v2 := 0.0, 0.0
	if l.Nodes[0] != 0 {
		v1 = voltages[l.Nodes[0]]
	}
	if l.Nodes[1] != 0 {
		v2 = voltages[l.Nodes[1]]
	}

	vd := v1 - v2
	dt := status.TimeStep

	// 상태 업데이트 방법에 따라 다르게 처리
	switch status.Method {
	case BE:
		l.Current0 = l.Current1 + (dt/l.Value)*vd
	case TR:
		l.Current0 = l.Current1 + (dt/(2.0*l.Value))*(vd+l.Voltage1)
	default:
		l.Current0 = l.Current1 + (dt/l.Value)*vd
	}

	// 상태 저장
	for i := len(l.dState) - 1; i > 0; i-- {
		l.dState[i] = l.dState[i-1]
	}
	l.dState[0] = l.Current0
}

func (l *Inductor) UpdateState(voltages []float64, status *CircuitStatus) {
	v1, v2 := 0.0, 0.0
	if l.Nodes[0] != 0 {
		v1 = voltages[l.Nodes[0]]
	}
	if l.Nodes[1] != 0 {
		v2 = voltages[l.Nodes[1]]
	}

	l.Voltage1 = l.Voltage0
	l.Voltage0 = v1 - v2
	l.Current1 = l.Current0
	l.prevDelta = status.TimeStep
}

func (l *Inductor) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	dt := status.TimeStep

	// SPICE3F5 방식의 LTE 계산
	switch status.Method {
	case BE:
		// BE는 1차 정확도
		if len(l.dState) < 2 {
			return 0.0
		}
		return math.Abs(l.dState[0]-l.dState[1]) / dt
	case TR:
		// TR은 2차 정확도
		if len(l.dState) < 3 {
			return 0.0
		}
		// 2차 도함수 근사
		d2i := (l.dState[0] - 2*l.dState[1] + l.dState[2]) / (dt * dt)
		return math.Abs(d2i * dt * dt / 12.0)
	default:
		return 0.0
	}
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

func (l *Inductor) BranchIndex() int {
	return l.branchIdx
}

func (l *Inductor) SetBranchIndex(idx int) {
	l.branchIdx = idx
}

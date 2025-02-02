package device

import (
	"toy-spice/pkg/matrix"
)

type Device interface {
	GetName() string
	GetType() string
	GetNodeNames() []string
	GetNodes() []int
	Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error
	GetValue() float64
	SetNodes(nodes []int)
}

type BaseDevice struct {
	Name      string
	Nodes     []int
	Value     float64
	NodeNames []string
}

type ModelParam struct {
	Type   string
	Name   string
	Params map[string]float64
}

type ACElement interface {
	StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error
}

type TimeDependent interface {
	SetTimeStep(dt float64)
	UpdateState(voltages []float64, status *CircuitStatus)
	CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64
}

type NonLinear interface {
	LoadConductance(matrix matrix.DeviceMatrix) error
	LoadCurrent(matrix matrix.DeviceMatrix) error
	UpdateVoltages(voltages []float64) error
}

type InductorComponent interface {
	Device
	GetValue() float64
	GetCurrent() float64
	GetPreviousCurrent() float64
	GetVoltage() float64
	GetPreviousVoltage() float64
	GetNodes() []int
}

type SourceType int

const (
	DC SourceType = iota
	SIN
	PULSE
	PWL
)

type AnalysisMode int

const (
	OperatingPointAnalysis AnalysisMode = iota
	TransientAnalysis
	ACAnalysis
	DCSweep
)

const (
	BE = iota // Backward Euler
	TR        // Trapezoidal
)

const (
	NormalMode = iota
	PredictMode
)

type CircuitStatus struct {
	Time      float64
	TimeStep  float64
	Gmin      float64
	Mode      AnalysisMode
	Method    int // BE or TR
	IntegMode int // Normal or Predict mode
	Temp      float64
	Order     int
	MaxOrder  int
	Frequency float64 // AC frequency
}

func (d *BaseDevice) GetName() string {
	return d.Name
}

func (d *BaseDevice) GetNodes() []int {
	return d.Nodes
}

func (d *BaseDevice) GetNodeNames() []string {
	return d.NodeNames
}

func (d *BaseDevice) GetValue() float64 {
	return d.Value
}

func (d *BaseDevice) SetNodes(nodes []int) {
	d.Nodes = nodes
}

func NewBaseDevice(name string, value float64, nodeNames []string, devType string) *BaseDevice {
	return &BaseDevice{
		Name:      name,
		Value:     value,
		NodeNames: nodeNames,
		Nodes:     make([]int, len(nodeNames)),
	}
}

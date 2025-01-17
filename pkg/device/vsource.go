package device

import (
	"math"
	"toy-spice/pkg/matrix"
)

type VoltageType int

const (
	DC VoltageType = iota
	SIN
	PULSE
	PWL
)

type ACVoltageSource struct {
	acMag   float64
	acPhase float64
}

type VoltageSource struct {
	BaseDevice
	vtype VoltageType
	// DC, common params
	dcValue float64
	// SIN params
	amplitude float64
	freq      float64
	phase     float64
	// PULSE params
	v1     float64
	v2     float64
	delay  float64
	rise   float64
	fall   float64
	pWidth float64
	period float64
	// PWL params
	times  []float64
	values []float64
	// AC params
	acMag   float64
	acPhase float64
	// Branch index for MNA
	branchIdx int
}

func NewDCVoltageSource(name string, nodeNames []string, value float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     value,
		},
		vtype:   DC,
		dcValue: value,
	}
}

func NewSinVoltageSource(name string, nodeNames []string, offset, amplitude, freq, phase float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     offset,
		},
		vtype:     SIN,
		dcValue:   offset,
		amplitude: amplitude,
		freq:      freq,
		phase:     phase,
	}
}

func NewDCVoltageSourceNotUse(name string, nodeNames []string, value float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: *NewBaseDevice(name, value, nodeNames, "V"),
		vtype:      DC,
		dcValue:    value,
	}
}

func NewSinVoltageSourceNotUse(name string, nodeNames []string, offset, amplitude, freq, phase float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: *NewBaseDevice(name, offset, nodeNames, "V"),
		vtype:      SIN,
		dcValue:    offset,
		amplitude:  amplitude,
		freq:       freq,
		phase:      phase,
	}
}

func NewACVoltageSource(name string, nodeNames []string, dcValue, acMag, acPhase float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     dcValue,
		},
		vtype:   DC,
		dcValue: dcValue,
		acMag:   acMag,
		acPhase: acPhase,
	}
}

func (v *VoltageSource) GetVoltage(t float64) float64 {
	switch v.vtype {
	case DC:
		return v.dcValue
	case SIN:
		phaseRad := v.phase * math.Pi / 180.0
		return v.dcValue + v.amplitude*math.Sin(2.0*math.Pi*v.freq*t+phaseRad)
	case PULSE:
		return v.getPulseVoltage()
	case PWL:
		return v.getPWLVoltage()
	default:
		return 0
	}
}

func (v *VoltageSource) GetType() string { return "V" }

func (v *VoltageSource) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if status.Mode == ACAnalysis {
		return v.StampAC(matrix, status)
	}

	n1, n2 := v.Nodes[0], v.Nodes[1]
	bIdx := v.branchIdx

	// v1 - v2 = V
	if n1 != 0 {
		matrix.AddElement(bIdx, n1, 1) // v1 coefficient
		matrix.AddElement(n1, bIdx, 1) // n1 current
	}
	if n2 != 0 {
		matrix.AddElement(bIdx, n2, -1) // -v2 coefficient
		matrix.AddElement(n2, bIdx, -1) // n2 current
	}

	voltage := v.GetVoltage(status.Time)
	matrix.AddRHS(bIdx, voltage)
	return nil
}

// Stamp for AC analysis
func (v *VoltageSource) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := v.Nodes[0], v.Nodes[1]
	bIdx := v.branchIdx

	acPhaseRad := v.acPhase * math.Pi / 180.0
	voltageReal := v.acMag * math.Cos(acPhaseRad)
	voltageImag := v.acMag * math.Sin(acPhaseRad)

	if n1 != 0 {
		matrix.AddComplexElement(bIdx, n1, 1.0, 0.0)
		matrix.AddComplexElement(n1, bIdx, 1.0, 0.0)
	}
	if n2 != 0 {
		matrix.AddComplexElement(bIdx, n2, -1.0, 0.0)
		matrix.AddComplexElement(n2, bIdx, -1.0, 0.0)
	}

	matrix.AddComplexRHS(bIdx, voltageReal, voltageImag)

	return nil
}

func (v *VoltageSource) getPulseVoltage() float64 {
	// PULSE는 나중에
	return v.v1
}

func (v *VoltageSource) getPWLVoltage() float64 {
	// PWL은 나중에
	return 0
}

func (v *VoltageSource) BranchIndex() int {
	return v.branchIdx
}

func (v *VoltageSource) SetBranchIndex(idx int) {
	v.branchIdx = idx
}

func (v *VoltageSource) SetValue(value float64) {
	v.Value = value
	v.dcValue = value
}

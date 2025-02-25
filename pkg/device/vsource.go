package device

import (
	"math"

	"github.com/edp1096/toy-spice/pkg/matrix"
)

type VoltageSource struct {
	BaseDevice
	vtype SourceType
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

func NewPulseVoltageSource(name string, nodeNames []string, v1, v2, delay, rise, fall, pWidth, period float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     v1,
		},
		vtype:  PULSE,
		v1:     v1,
		v2:     v2,
		delay:  delay,
		rise:   rise,
		fall:   fall,
		pWidth: pWidth,
		period: period,
	}
}

func NewPWLVoltageSource(name string, nodeNames []string, times []float64, values []float64) *VoltageSource {
	return &VoltageSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     values[0], // First value as initial value
		},
		vtype:  PWL,
		times:  times,
		values: values,
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
		return v.getPulseVoltage(t)
	case PWL:
		return v.getPWLVoltage(t)
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

	// Convert AC phase to rad
	phaseRad := v.acPhase * math.Pi / 180.0

	// Set complex voltage: magnitude * (cos(θ) + j*sin(θ))
	voltageReal := v.acMag * math.Cos(phaseRad)
	voltageImag := v.acMag * math.Sin(phaseRad)

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

func (v *VoltageSource) getPulseVoltage(t float64) float64 {
	if t < v.delay {
		return v.v1
	}

	t = t - v.delay
	if v.period > 0 {
		t = math.Mod(t, v.period)
	}

	if t < v.rise {
		if v.rise == 0 {
			return v.v2
		}
		return v.v1 + (v.v2-v.v1)*t/v.rise
	}

	if t < v.rise+v.pWidth {
		return v.v2
	}

	fallStart := v.rise + v.pWidth
	if t < fallStart+v.fall {
		if v.fall == 0 {
			return v.v1
		}
		return v.v2 - (v.v2-v.v1)*(t-fallStart)/v.fall
	}

	return v.v1
}

func (v *VoltageSource) getPWLVoltage(t float64) float64 {
	if t <= v.times[0] {
		return v.values[0]
	}

	lastIdx := len(v.times) - 1
	if t >= v.times[lastIdx] {
		return v.values[lastIdx]
	}

	for i := 1; i < len(v.times); i++ {
		if t <= v.times[i] {
			t1, t2 := v.times[i-1], v.times[i]
			v1, v2 := v.values[i-1], v.values[i]
			slope := (v2 - v1) / (t2 - t1)
			return v1 + slope*(t-t1)
		}
	}

	return v.values[lastIdx] // Must not reach
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

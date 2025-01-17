package device

import (
	"math"
	"toy-spice/pkg/matrix"
)

type CurrentSource struct {
	BaseDevice
	ctype SourceType
	// DC, common params
	dcValue float64
	// SIN params
	amplitude float64
	freq      float64
	phase     float64
	// PULSE params
	i1     float64
	i2     float64
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
}

func NewDCCurrentSource(name string, nodeNames []string, value float64) *CurrentSource {
	return &CurrentSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     value,
		},
		ctype:   DC,
		dcValue: value,
	}
}

func NewSinCurrentSource(name string, nodeNames []string, offset, amplitude, freq, phase float64) *CurrentSource {
	return &CurrentSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     offset,
		},
		ctype:     SIN,
		dcValue:   offset,
		amplitude: amplitude,
		freq:      freq,
		phase:     phase,
	}
}

func NewPulseCurrentSource(name string, nodeNames []string, i1, i2, delay, rise, fall, pWidth, period float64) *CurrentSource {
	return &CurrentSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     i1,
		},
		ctype:  PULSE,
		i1:     i1,
		i2:     i2,
		delay:  delay,
		rise:   rise,
		fall:   fall,
		pWidth: pWidth,
		period: period,
	}
}

func NewPWLCurrentSource(name string, nodeNames []string, times []float64, values []float64) *CurrentSource {
	return &CurrentSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     values[0],
		},
		ctype:  PWL,
		times:  times,
		values: values,
	}
}

func NewACCurrentSource(name string, nodeNames []string, dcValue, acMag, acPhase float64) *CurrentSource {
	return &CurrentSource{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     dcValue,
		},
		ctype:   DC,
		dcValue: dcValue,
		acMag:   acMag,
		acPhase: acPhase,
	}
}

func (i *CurrentSource) GetCurrent(t float64) float64 {
	switch i.ctype {
	case DC:
		return i.dcValue
	case SIN:
		phaseRad := i.phase * math.Pi / 180.0
		return i.dcValue + i.amplitude*math.Sin(2.0*math.Pi*i.freq*t+phaseRad)
	case PULSE:
		return i.getPulseCurrent(t)
	case PWL:
		return i.getPWLCurrent(t)
	default:
		return 0
	}
}

func (i *CurrentSource) GetType() string { return "I" }

// Stamp for DC, transient analysis
func (i *CurrentSource) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if status.Mode == ACAnalysis {
		return i.StampAC(matrix, status)
	}

	n1, n2 := i.Nodes[0], i.Nodes[1]
	current := i.GetCurrent(status.Time)

	// By KCL, Current flow into n1 and out of n2
	if n1 != 0 {
		matrix.AddRHS(n1, current) // Current flow into n1 (+)
	}
	if n2 != 0 {
		matrix.AddRHS(n2, -current) // Current flow out of n2 (-)
	}

	return nil
}

// Stamp for AC analysis
func (i *CurrentSource) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := i.Nodes[0], i.Nodes[1]

	acPhaseRad := i.acPhase * math.Pi / 180.0
	currentReal := i.acMag * math.Cos(acPhaseRad)
	currentImag := i.acMag * math.Sin(acPhaseRad)

	if n1 != 0 {
		matrix.AddComplexRHS(n1, currentReal, currentImag)
	}
	if n2 != 0 {
		matrix.AddComplexRHS(n2, -currentReal, -currentImag)
	}

	return nil
}

func (i *CurrentSource) getPulseCurrent(t float64) float64 {
	if t < i.delay {
		return i.i1
	}

	t = t - i.delay
	if i.period > 0 {
		t = math.Mod(t, i.period)
	}

	if t < i.rise {
		if i.rise == 0 {
			return i.i2
		}
		return i.i1 + (i.i2-i.i1)*t/i.rise
	}

	if t < i.rise+i.pWidth {
		return i.i2
	}

	fallStart := i.rise + i.pWidth
	if t < fallStart+i.fall {
		if i.fall == 0 {
			return i.i1
		}
		return i.i2 - (i.i2-i.i1)*(t-fallStart)/i.fall
	}

	return i.i1
}

func (i *CurrentSource) getPWLCurrent(t float64) float64 {
	if t <= i.times[0] {
		return i.values[0]
	}

	lastIdx := len(i.times) - 1
	if t >= i.times[lastIdx] {
		return i.values[lastIdx]
	}

	for idx := 1; idx < len(i.times); idx++ {
		if t <= i.times[idx] {
			t1, t2 := i.times[idx-1], i.times[idx]
			i1, i2 := i.values[idx-1], i.values[idx]
			slope := (i2 - i1) / (t2 - t1)
			return i1 + slope*(t-t1)
		}
	}

	return i.values[lastIdx] // Must not reach
}

func (i *CurrentSource) SetValue(value float64) {
	i.Value = value
	i.dcValue = value
}

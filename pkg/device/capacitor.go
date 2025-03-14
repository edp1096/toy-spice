package device

import (
	"math"

	"github.com/edp1096/toy-spice/pkg/matrix"
)

type Capacitor struct {
	BaseDevice
	Voltage0 float64 // Current voltage
	Voltage1 float64 // Previous voltage
	current0 float64 // Current current
	current1 float64 // Previous current
	charge0  float64 // Current charge
	charge1  float64 // Previous charge

	Tc1  float64
	Tc2  float64
	Tnom float64
}

var _ TimeDependent = (*Capacitor)(nil)

func NewCapacitor(name string, nodeNames []string, value float64) *Capacitor {
	return &Capacitor{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
			Value:     value,
		},
		Tc1:  0.0,
		Tc2:  0.0,
		Tnom: 300.15,
	}
}

func (c *Capacitor) GetType() string { return "C" }

func (c *Capacitor) SetTimeStep(dt float64, status *CircuitStatus) { status.TimeStep = dt }

func (c *Capacitor) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	n1, n2 := c.Nodes[0], c.Nodes[1]
	adjustedC := c.temperatureAdjustedValue(status.Temp)

	switch status.Mode {
	case ACAnalysis:
		omega := 2 * math.Pi * status.Frequency
		capConductanceReal := 0.0
		// capConductanceImag := omega * c.Value // C * jω
		capConductanceImag := omega * adjustedC // C * jω

		if n1 != 0 {
			matrix.AddComplexElement(n1, n1, capConductanceReal, capConductanceImag)
			if n2 != 0 {
				matrix.AddComplexElement(n1, n2, -capConductanceReal, -capConductanceImag)
			}
		}
		if n2 != 0 {
			matrix.AddComplexElement(n2, n2, capConductanceReal, capConductanceImag)
			if n1 != 0 {
				matrix.AddComplexElement(n2, n1, -capConductanceReal, -capConductanceImag)
			}
		}

	case OperatingPointAnalysis:
		gmin := status.Gmin
		if gmin < 1e-12 {
			gmin = 1e-12
		}
		if n1 != 0 {
			matrix.AddElement(n1, n1, gmin)
			if n2 != 0 {
				matrix.AddElement(n1, n2, -gmin)
			}
		}
		if n2 != 0 {
			matrix.AddElement(n2, n2, gmin)
			if n1 != 0 {
				matrix.AddElement(n2, n1, -gmin)
			}
		}

	case TransientAnalysis:
		dt := status.TimeStep
		// geq := 2.0 * adjustedC / dt
		// ceq := geq*c.Voltage0/2.0 + c.current1
		geq := adjustedC / dt
		ceq := c.charge1 / dt

		if n1 != 0 {
			matrix.AddElement(n1, n1, geq)
			if n2 != 0 {
				matrix.AddElement(n1, n2, -geq)
			}
			matrix.AddRHS(n1, ceq)
		}
		if n2 != 0 {
			matrix.AddElement(n2, n2, geq)
			if n1 != 0 {
				matrix.AddElement(n2, n1, -geq)
			}
			matrix.AddRHS(n2, -ceq)
		}
	}

	return nil
}

func (c *Capacitor) LoadState(voltages []float64, status *CircuitStatus) {
	v1 := 0.0
	if c.Nodes[0] != 0 {
		v1 = voltages[c.Nodes[0]]
	}
	v2 := 0.0
	if c.Nodes[1] != 0 {
		v2 = voltages[c.Nodes[1]]
	}
	vd := v1 - v2

	// 전류는 i = C * dv/dt
	c.current0 = c.Value * (vd - c.Voltage0) / status.TimeStep
}

func (c *Capacitor) UpdateStateNotUse(voltages []float64, status *CircuitStatus) {
	v1 := 0.0
	if c.Nodes[0] != 0 {
		v1 = voltages[c.Nodes[0]]
	}
	v2 := 0.0
	if c.Nodes[1] != 0 {
		v2 = voltages[c.Nodes[1]]
	}
	vd := v1 - v2
	dt := status.TimeStep

	if status.IntegMode == PredictMode {
		// Predict Mode - copy previous state
		c.charge0 = c.charge1
		c.current0 = c.current1
		c.Voltage0 = c.Voltage1
	} else {
		// Normal Mode - update state to previous voltage
		c.charge0 = c.charge1 + (c.current0+c.current1)*dt/2.0
		c.current0 = c.Value * (vd - c.Voltage1) / dt
		c.Voltage0 = vd
	}

	c.charge1 = c.charge0
	c.Voltage1 = c.Voltage0
	c.current1 = c.current0
}

func (c *Capacitor) UpdateState(voltages []float64, status *CircuitStatus) {
	v1 := 0.0
	if c.Nodes[0] != 0 {
		v1 = voltages[c.Nodes[0]]
	}
	v2 := 0.0
	if c.Nodes[1] != 0 {
		v2 = voltages[c.Nodes[1]]
	}
	vd := v1 - v2

	c.charge1 = c.charge0
	c.charge0 = c.Value * vd

	c.Voltage1 = c.Voltage0
	c.Voltage0 = vd
}

func (c *Capacitor) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	qNew := c.Value * c.Voltage0
	qOld := c.Value * c.Voltage1

	return math.Abs(qNew-qOld) / (2.0 * status.TimeStep)
}

func (c *Capacitor) temperatureAdjustedValue(temp float64) float64 {
	dt := temp - c.Tnom
	factor := 1.0 + c.Tc1*dt + c.Tc2*dt*dt
	return c.Value * factor
}

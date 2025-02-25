package analysis

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
)

type ACAnalysis struct {
	BaseAnalysis
	op          *OperatingPoint
	startFreq   float64
	stopFreq    float64
	numPoints   int
	pointsType  string // "DEC", "OCT", "LIN"
	frequencies []float64
}

func NewAC(fStart, fStop float64, nPoints int, pType string) *ACAnalysis {
	return &ACAnalysis{
		BaseAnalysis: *NewBaseAnalysis(),
		op:           NewOP(),
		startFreq:    fStart,
		stopFreq:     fStop,
		numPoints:    nPoints,
		pointsType:   pType,
	}
}

func (ac *ACAnalysis) Setup(ckt *circuit.Circuit) error {
	var err error

	ac.Circuit = ckt

	err = ac.op.Setup(ckt)
	if err != nil {
		return fmt.Errorf("operating point setup error: %v", err)
	}
	err = ac.op.Execute()
	if err != nil {
		return fmt.Errorf("operating point analysis error: %v", err)
	}

	ac.generateFrequencyPoints()

	return nil
}

func (ac *ACAnalysis) Execute() error {
	if ac.Circuit == nil {
		return fmt.Errorf("circuit not set")
	}

	for _, freq := range ac.frequencies {
		ac.Circuit.Status = &device.CircuitStatus{
			Frequency: freq,
			Mode:      device.ACAnalysis,
			Temp:      300.15, // 27 = 300.15K
		}

		mat := ac.Circuit.GetMatrix()
		mat.Clear()
		err := ac.Circuit.Stamp(ac.Circuit.Status)
		if err != nil {
			return fmt.Errorf("stamping error at f=%g: %v", freq, err)
		}

		err = mat.Solve()
		if err != nil {
			return fmt.Errorf("matrix solve error at f=%g: %v", freq, err)
		}

		solution := make(map[string]complex128)

		// Node voltage
		for name, nodeIdx := range ac.Circuit.GetNodeMap() {
			if nodeIdx > 0 {
				real, imag := mat.GetComplexSolution(nodeIdx)
				solution[fmt.Sprintf("V(%s)", name)] = complex(real, imag)
			}
		}

		// Branch current
		for _, dev := range ac.Circuit.GetDevices() {
			if v, ok := dev.(*device.VoltageSource); ok {
				bIdx := v.BranchIndex()
				real, imag := mat.GetComplexSolution(bIdx)
				solution[fmt.Sprintf("I(%s)", dev.GetName())] = complex(real, imag)
			}
		}

		ac.StoreACResult(freq, solution)
	}

	return nil
}

func (ac *ACAnalysis) generateFrequencyPoints() {
	ac.frequencies = make([]float64, ac.numPoints)

	switch ac.pointsType {
	case "DEC": // Decade
		logStart := math.Log10(ac.startFreq)
		logStop := math.Log10(ac.stopFreq)
		step := (logStop - logStart) / float64(ac.numPoints-1)
		for i := range ac.numPoints {
			ac.frequencies[i] = math.Pow(10, logStart+float64(i)*step)
		}

	case "OCT": // Octave
		logStart := math.Log2(ac.startFreq)
		logStop := math.Log2(ac.stopFreq)
		step := (logStop - logStart) / float64(ac.numPoints-1)
		for i := range ac.numPoints {
			ac.frequencies[i] = math.Pow(2, logStart+float64(i)*step)
		}

	case "LIN": // Linear
		step := (ac.stopFreq - ac.startFreq) / float64(ac.numPoints-1)
		for i := range ac.numPoints {
			ac.frequencies[i] = ac.startFreq + float64(i)*step
		}
	}
}

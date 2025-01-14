package analysis

import (
	"math"
	"math/cmplx"

	"toy-spice/pkg/circuit"
	"toy-spice/pkg/util"
)

const (
	OP int = iota
	TRAN
	AC
)

type Analysis interface {
	Setup(ckt *circuit.Circuit) error
	Execute() error
	GetResults() map[string][]float64
}

type BaseAnalysis struct {
	Circuit     *circuit.Circuit
	results     map[string][]float64 // key: variable name, value: result by time
	convergence struct {
		maxIter int
		abstol  float64
		reltol  float64
		gmin    float64
	}
}

func NewBaseAnalysis() *BaseAnalysis {
	ba := &BaseAnalysis{results: make(map[string][]float64)}

	ba.convergence.maxIter = 100
	ba.convergence.abstol = 1e-12
	ba.convergence.reltol = 1e-6
	ba.convergence.gmin = 1e-12

	return ba
}

func (a *BaseAnalysis) CheckConvergence(oldSol, newSol []float64) bool {
	if len(oldSol) != len(newSol) {
		return false
	}

	for i := range oldSol {
		diff := math.Abs(newSol[i] - oldSol[i])
		if diff > a.convergence.abstol &&
			diff > a.convergence.reltol*math.Abs(newSol[i]) {
			return false
		}
	}
	return true
}

func (a *BaseAnalysis) StoreTimeResult(time float64, solution map[string]float64) {
	// Ignore same time
	if len(a.results["TIME"]) > 0 {
		lastTime := a.results["TIME"][len(a.results["TIME"])-1]
		if time == lastTime {
			return
		}
		// Compare rounded string. 1.999999e-05 == 2.000000e-05
		if util.FormatValueFactor(time, "s") == util.FormatValueFactor(lastTime, "s") {
			return
		}
	}

	if _, exists := a.results["TIME"]; !exists {
		a.results["TIME"] = make([]float64, 0)
	}
	a.results["TIME"] = append(a.results["TIME"], time)

	for name, value := range solution {
		if _, exists := a.results[name]; !exists {
			a.results[name] = make([]float64, 0)
		}
		a.results[name] = append(a.results[name], value)
	}
}

func (a *BaseAnalysis) StoreACResult(freq float64, solution map[string]complex128) {
	// Frequency
	if _, exists := a.results["FREQ"]; !exists {
		a.results["FREQ"] = make([]float64, 0)
	}
	a.results["FREQ"] = append(a.results["FREQ"], freq)

	for name, value := range solution {
		// Magnitude
		magName := name + "_MAG"
		if _, exists := a.results[magName]; !exists {
			a.results[magName] = make([]float64, 0)
		}
		magnitude := cmplx.Abs(value)
		a.results[magName] = append(a.results[magName], magnitude)

		// Phase - degree
		phaseName := name + "_PHASE"
		if _, exists := a.results[phaseName]; !exists {
			a.results[phaseName] = make([]float64, 0)
		}
		phase := cmplx.Phase(value) * 180.0 / math.Pi
		a.results[phaseName] = append(a.results[phaseName], phase)
	}
}

func (a *BaseAnalysis) GetResults() map[string][]float64 {
	return a.results
}

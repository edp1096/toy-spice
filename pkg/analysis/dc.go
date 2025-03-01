package analysis

import (
	"fmt"

	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
)

type DCSweep struct {
	BaseAnalysis
	sourceNames []string    // Names of voltage/current sources to sweep
	startVals   []float64   // Start values for each source
	stopVals    []float64   // Stop values for each source
	increments  []float64   // Incremental value of steps for each source
	sweepVals   [][]float64 // Generated sweep values for each source
	origVals    []float64   // Original values of the sources
}

func NewDCSweep(sources []string, starts, stops []float64, numSteps []float64) *DCSweep {
	if len(sources) != len(starts) || len(sources) != len(stops) || len(sources) != len(numSteps) {
		panic("inconsistent parameter lengths")
	}

	dc := &DCSweep{
		BaseAnalysis: *NewBaseAnalysis(),
		sourceNames:  sources,
		startVals:    starts,
		stopVals:     stops,
		increments:   numSteps,
		sweepVals:    make([][]float64, len(sources)),
		origVals:     make([]float64, len(sources)),
	}

	// Generate sweep values for each source
	for i := range sources {
		sweep := make([]float64, 0)
		for v := dc.startVals[i]; v <= dc.stopVals[i]; v += dc.increments[i] {
			sweep = append(sweep, v)
		}
		dc.sweepVals[i] = sweep
	}

	return dc
}

func (dc *DCSweep) Setup(ckt *circuit.Circuit) error {
	dc.Circuit = ckt

	// Store original source values
	for i, name := range dc.sourceNames {
		found := false
		for _, dev := range ckt.GetDevices() {
			if dev.GetName() == name {
				if v, ok := dev.(*device.VoltageSource); ok {
					dc.origVals[i] = v.GetValue()
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("source %s not found", name)
		}
	}

	return nil
}

func (dc *DCSweep) Execute() error {
	if dc.Circuit == nil {
		return fmt.Errorf("circuit not set")
	}

	// Single source sweep
	if len(dc.sourceNames) == 1 {
		return dc.singleSweep()
	}

	// Nested sweep (currently supporting up to 2 sources)
	if len(dc.sourceNames) == 2 {
		return dc.nestedSweep()
	}

	return fmt.Errorf("unsupported number of sweep sources: %d", len(dc.sourceNames))
}

func (dc *DCSweep) singleSweep() error {
	var err error

	sourceName := dc.sourceNames[0]

	// Find the source device
	var source *device.VoltageSource
	for _, dev := range dc.Circuit.GetDevices() {
		if dev.GetName() == sourceName {
			if v, ok := dev.(*device.VoltageSource); ok {
				source = v
				break
			}
		}
	}

	if source == nil {
		return fmt.Errorf("source %s not found", sourceName)
	}

	// Perform sweep
	for _, val := range dc.sweepVals[0] {
		source.SetValue(val)

		// Run operating point analysis
		status := &device.CircuitStatus{
			Mode: device.OperatingPointAnalysis,
			Temp: 300.15,
			Gmin: dc.convergence.gmin,
		}

		mat := dc.Circuit.GetMatrix()
		mat.Clear()

		err = dc.Circuit.Stamp(status)
		if err != nil {
			return fmt.Errorf("stamping error at %s=%g: %v", sourceName, val, err)
		}

		err = dc.doNRiter(0, dc.convergence.maxIter)
		if err != nil {
			return fmt.Errorf("convergence error at %s=%g: %v", sourceName, val, err)
		}

		// Store results
		solution := dc.Circuit.GetSolution()
		dc.StoreResult(val, solution)
	}

	source.SetValue(dc.origVals[0])

	return nil
}

func (dc *DCSweep) doNRiter(gmin float64, maxIter int) error {
	var err error

	ckt := dc.Circuit
	mat := ckt.GetMatrix()
	var oldSolution []float64

	cktStatus := &device.CircuitStatus{
		Mode: device.OperatingPointAnalysis,
		Temp: 300.15,
		Gmin: gmin,
	}

	for iter := range maxIter {
		mat.Clear()
		if iter > 0 {
			err := ckt.UpdateNonlinearVoltages(oldSolution)
			if err != nil {
				return fmt.Errorf("updating nonlinear voltages: %v", err)
			}
		}

		err = ckt.Stamp(cktStatus)
		if err != nil {
			return fmt.Errorf("stamping error: %v", err)
		}

		mat.LoadGmin(gmin)
		err := mat.Solve()
		if err != nil {
			return fmt.Errorf("matrix solve error: %v", err)
		}

		solution := mat.Solution()
		if iter > 0 && dc.CheckConvergence(oldSolution, solution) {
			return nil
		}

		if oldSolution == nil {
			oldSolution = make([]float64, len(solution))
		}
		copy(oldSolution, solution)
	}

	return fmt.Errorf("failed to converge in %d iterations", maxIter)
}

func (dc *DCSweep) StoreResult(sweepVal float64, solution map[string]float64) {
	// Store sweep value
	if _, exists := dc.results["SWEEP1"]; !exists {
		dc.results["SWEEP1"] = make([]float64, 0)
	}
	dc.results["SWEEP1"] = append(dc.results["SWEEP1"], sweepVal)

	// Store node voltages and branch currents
	for name, value := range solution {
		if _, exists := dc.results[name]; !exists {
			dc.results[name] = make([]float64, 0)
		}
		dc.results[name] = append(dc.results[name], value)
	}
}

func (dc *DCSweep) nestedSweep() error {
	var err error

	source1Name := dc.sourceNames[0]
	source2Name := dc.sourceNames[1]

	// Find source devices
	var source1, source2 *device.VoltageSource
	for _, dev := range dc.Circuit.GetDevices() {
		if dev.GetName() == source1Name {
			if v, ok := dev.(*device.VoltageSource); ok {
				source1 = v
			}
		}
		if dev.GetName() == source2Name {
			if v, ok := dev.(*device.VoltageSource); ok {
				source2 = v
			}
		}
	}

	if source1 == nil || source2 == nil {
		return fmt.Errorf("source not found")
	}

	// Nested sweep
	for _, val1 := range dc.sweepVals[0] {
		source1.SetValue(val1)

		for _, val2 := range dc.sweepVals[1] {
			source2.SetValue(val2)

			// Run operating point analysis
			status := &device.CircuitStatus{
				Mode: device.OperatingPointAnalysis,
				Temp: 300.15,
				Gmin: dc.convergence.gmin,
			}

			mat := dc.Circuit.GetMatrix()
			mat.Clear()

			err = dc.Circuit.Stamp(status)
			if err != nil {
				return fmt.Errorf("stamping error at %s=%g, %s=%g: %v",
					source1Name, val1, source2Name, val2, err)
			}

			err = dc.doNRiter(0, dc.convergence.maxIter)
			if err != nil {
				return fmt.Errorf("convergence error at %s=%g, %s=%g: %v",
					source1Name, val1, source2Name, val2, err)
			}

			// Store results with both sweep values
			solution := dc.Circuit.GetSolution()
			dc.StoreNestedResult(val1, val2, solution)
		}
	}

	// Restore original values
	source1.SetValue(dc.origVals[0])
	source2.SetValue(dc.origVals[1])

	return nil
}

func (dc *DCSweep) StoreNestedResult(val1, val2 float64, solution map[string]float64) {
	// Store sweep values
	if _, exists := dc.results["SWEEP1"]; !exists {
		dc.results["SWEEP1"] = make([]float64, 0)
		dc.results["SWEEP2"] = make([]float64, 0)
	}
	dc.results["SWEEP1"] = append(dc.results["SWEEP1"], val1)
	dc.results["SWEEP2"] = append(dc.results["SWEEP2"], val2)

	// Store all node voltages and branch currents
	for name, value := range solution {
		if _, exists := dc.results[name]; !exists {
			dc.results[name] = make([]float64, 0)
		}
		dc.results[name] = append(dc.results[name], value)
	}
}

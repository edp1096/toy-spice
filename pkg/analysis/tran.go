package analysis

import (
	"fmt"
	"math"
	"toy-spice/pkg/circuit"
	"toy-spice/pkg/device"
)

type Transient struct {
	BaseAnalysis
	op        *OperatingPoint
	time      float64
	startTime float64
	stopTime  float64
	timeStep  float64
	maxStep   float64
	minStep   float64
	useUIC    bool

	// Local Truncation Error
	order     int     // ODE (1=BE, 2=TR)
	trtol     float64 // truncation error tolerance (SPICE3F5 default: 7)
	firstTime bool
	prevStep  float64
}

func NewTransient(tStart, tStop, tStep, tMax float64, uic bool) *Transient {
	minStep := tStep / 50.0
	if tMax == 0 {
		tMax = tStep
	}
	return &Transient{
		BaseAnalysis: *NewBaseAnalysis(),
		op:           NewOP(),
		startTime:    tStart,
		stopTime:     tStop,
		timeStep:     tStep,
		maxStep:      tMax,
		minStep:      minStep,
		useUIC:       uic,
		time:         0,
		order:        1,   // BE
		trtol:        7.0, // SPICE3F5 default
		firstTime:    true,
	}
}

func (tr *Transient) Setup(ckt *circuit.Circuit) error {
	tr.Circuit = ckt

	if !tr.useUIC {
		if err := tr.op.Setup(ckt); err != nil {
			return fmt.Errorf("operating point setup error: %v", err)
		}
		if err := tr.op.Execute(); err != nil {
			return fmt.Errorf("operating point analysis error: %v", err)
		}
	}

	tr.Circuit.SetTimeStep(tr.timeStep)
	return nil
}

func (tr *Transient) Execute() error {
	if tr.Circuit == nil {
		return fmt.Errorf("circuit not set")
	}

	for tr.time < tr.stopTime {
		nextTime := tr.time + tr.timeStep
		if nextTime > tr.stopTime {
			nextTime = tr.stopTime
			tr.timeStep = nextTime - tr.time
		}

		status := &device.CircuitStatus{
			Time:     tr.time,
			TimeStep: tr.timeStep,
			Mode:     device.TransientAnalysis,
			Method:   tr.order,
			Temp:     300.15, // 27 = 300.15K
			Gmin:     tr.convergence.gmin,
		}
		tr.Circuit.Status = status
		tr.Circuit.SetTimeStep(tr.timeStep)

		err := tr.doNRiter(0, tr.convergence.maxIter)
		if err != nil {
			if tr.timeStep > tr.minStep {
				tr.timeStep /= 2
				continue
			}
			return fmt.Errorf("failed to converge at t=%g", tr.time)
		}

		tr.Circuit.Update()
		tr.time = nextTime
		if tr.time >= tr.startTime {
			tr.StoreTimeResult(tr.time, tr.Circuit.GetSolution())
		}

		if tr.time < tr.stopTime {
			if tr.timeStep < tr.maxStep {
				tr.timeStep *= 1.1
				if tr.timeStep > tr.maxStep {
					tr.timeStep = tr.maxStep
				}
			}
		}
	}

	return nil
}

func (tr *Transient) doNRiter(gmin float64, maxIter int) error {
	ckt := tr.Circuit
	mat := ckt.GetMatrix()
	var oldSolution []float64
	cktStatus := &device.CircuitStatus{
		Time:     tr.time,
		TimeStep: tr.timeStep,
		Mode:     device.TransientAnalysis,
		Method:   tr.order,
		Temp:     300.15,
		Gmin:     gmin,
	}

	for iter := 0; iter < maxIter; iter++ {
		mat.Clear()
		if iter > 0 {
			if err := ckt.UpdateNonlinearVoltages(oldSolution); err != nil {
				return fmt.Errorf("updating nonlinear voltages: %v", err)
			}
		}

		if err := ckt.Stamp(cktStatus); err != nil {
			return fmt.Errorf("stamping error: %v", err)
		}
		mat.LoadGmin(gmin)
		if err := mat.Solve(); err != nil {
			return fmt.Errorf("matrix solve error: %v", err)
		}

		solution := mat.Solution()
		if iter > 0 {
			allConverged := true
			for i := 1; i < len(solution); i++ {
				diff := math.Abs(solution[i] - oldSolution[i])
				reltol := tr.convergence.reltol*math.Max(
					math.Abs(solution[i]),
					math.Abs(oldSolution[i])) + tr.convergence.abstol
				if diff > reltol {
					allConverged = false
					break
				}
			}
			if allConverged {
				return nil
			}
		}

		if oldSolution == nil {
			oldSolution = make([]float64, len(solution))
		}
		copy(oldSolution, solution)
	}

	return fmt.Errorf("failed to converge in %d iterations", maxIter)
}

func (tr *Transient) checkAcceptability() (bool, error) {
	if tr.firstTime {
		tr.firstTime = false
		tr.order = 2 // TR

		tol := tr.calculateTruncError()
		if tol > tr.trtol {
			tr.order = 1 // BE
			return true, nil
		}
		return true, nil
	}

	tol := tr.calculateTruncError()
	if tol >= 1.0 {
		return false, nil
	}

	return true, nil
}

func (tr *Transient) calculateTruncError() float64 {
	maxLTE := 0.0
	for _, dev := range tr.Circuit.GetDevices() {
		if td, ok := dev.(device.TimeDependent); ok {
			lte := td.CalculateLTE(tr.Circuit.GetSolution(), tr.Circuit.Status)
			if lte > maxLTE {
				maxLTE = lte
			}
		}
	}
	return maxLTE
}

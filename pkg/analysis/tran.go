package analysis

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
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
	if tStep > tStop/300 {
		tStep = tStop / 300
	}

	minStep := tStep / 50.0
	if tMax == 0 {
		tMax = tStep
	}

	analysisSettings := &Transient{
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

	return analysisSettings
}

func (tr *Transient) Setup(ckt *circuit.Circuit) error {
	var err error

	tr.Circuit = ckt

	if !tr.useUIC {
		err = tr.op.Setup(ckt)
		if err != nil {
			return fmt.Errorf("operating point setup error: %v", err)
		}
		err = tr.op.Execute()
		if err != nil {
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

	if !tr.useUIC {
		err := tr.op.Setup(tr.Circuit)
		if err != nil {
			return fmt.Errorf("operating point setup error: %v", err)
		}
		err = tr.op.Execute()
		if err != nil {
			return fmt.Errorf("operating point analysis error: %v", err)
		}
	}

	tr.timeStep = tr.minStep
	methodState := device.BE

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
			Method:   methodState,
			Temp:     300.15,
			Gmin:     tr.convergence.gmin,
		}
		tr.Circuit.Status = status

		err := tr.doNRiter(0, tr.convergence.maxIter)
		if err != nil {
			if tr.timeStep > tr.minStep {
				tr.timeStep /= 2
				continue
			}
			return fmt.Errorf("failed to converge at t=%g", tr.time)
		}

		lte := tr.calculateTruncError()
		if lte > tr.trtol {
			if tr.timeStep > tr.minStep {
				tr.timeStep /= 2
				continue
			}
		}

		// BE -> TR
		if methodState == device.BE && tr.time > 0 {
			if lte < tr.trtol/10 {
				methodState = device.TR
			}
		}

		tr.Circuit.LoadState()
		tr.Circuit.Update()
		tr.time = nextTime

		if tr.time >= tr.startTime {
			tr.StoreTimeResult(tr.time, tr.Circuit.GetSolution())
		}

		if tr.time < tr.stopTime && tr.timeStep < tr.maxStep {
			if lte < tr.trtol/100 {
				tr.timeStep = math.Min(tr.timeStep*2, tr.maxStep)
			} else {
				tr.timeStep = math.Min(tr.timeStep*1.1, tr.maxStep)
			}
		}
	}

	return nil
}

func (tr *Transient) doNRiter(gmin float64, maxIter int) error {
	var err error

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

	for iter := range maxIter {
		mat.Clear()
		if iter > 0 {
			err = ckt.UpdateNonlinearVoltages(oldSolution)
			if err != nil {
				return fmt.Errorf("updating nonlinear voltages: %v", err)
			}
		}

		err = ckt.Stamp(cktStatus)
		if err != nil {
			return fmt.Errorf("stamping error: %v", err)
		}
		mat.LoadGmin(gmin)
		err = mat.Solve()
		if err != nil {
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

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

		// 상태 설정
		status := &device.CircuitStatus{
			Time:     tr.time,
			TimeStep: tr.timeStep,
			Mode:     device.TransientAnalysis,
			Method:   tr.order,
			Temp:     300.0, // 기본 온도
			Gmin:     tr.convergence.gmin,
		}
		tr.Circuit.Status = status

		// Gmin stepping을 사용한 해 탐색
		gminValues := []float64{1e-2, 1e-3, 1e-4, 1e-5, 1e-6, 1e-7, 1e-8, 1e-9, 1e-10, 1e-11, tr.convergence.gmin}
		solved := false

		for _, gmin := range gminValues {
			status.Gmin = gmin
			err := tr.doNRiter(gmin, tr.convergence.maxIter)
			if err == nil {
				solved = true
				break
			}
		}

		if !solved {
			if tr.timeStep > tr.minStep {
				tr.timeStep /= 2
				continue
			}
			return fmt.Errorf("failed to converge at t=%g", tr.time)
		}

		// 해를 찾은 경우
		tr.Circuit.Update()
		tr.time = nextTime
		if tr.time >= tr.startTime {
			tr.StoreTimeResult(tr.time, tr.Circuit.GetSolution())
		}

		// 다음 스텝 준비
		if tr.time < tr.stopTime {
			if tr.timeStep < tr.maxStep {
				tr.timeStep *= 1.1 // 좀 더 보수적인 증가율
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
	var oldSolution map[string]float64
	cktStatus := &device.CircuitStatus{
		Time:     tr.time,
		TimeStep: tr.timeStep,
		Gmin:     gmin,
		Mode:     device.TransientAnalysis,
		Method:   tr.order, // BE or TR
	}

	for iter := 0; iter < maxIter; iter++ {
		mat.Clear()

		// First iteration have no previous solution so, skip
		if iter > 0 {
			if solution := mat.Solution(); solution != nil {
				if err := ckt.UpdateNonlinearVoltages(solution); err != nil {
					return fmt.Errorf("updating nonlinear voltages: %v", err)
				}
			}
		}

		if err := ckt.Stamp(cktStatus); err != nil {
			return fmt.Errorf("stamping error: %v", err)
		}
		mat.LoadGmin(gmin)

		if err := mat.Solve(); err != nil {
			return fmt.Errorf("matrix solve error: %v", err)
		}

		solution := ckt.GetSolution()

		if iter > 0 {
			allConverged := true
			for key, value := range solution {
				if oldValue, ok := oldSolution[key]; ok {
					diff := math.Abs(value - oldValue)
					reltol := tr.convergence.reltol*math.Max(
						math.Abs(value),
						math.Abs(oldValue)) + tr.convergence.abstol

					if diff > reltol {
						allConverged = false
						break
					}
				}
			}

			if allConverged {
				return nil
			}
		}

		if oldSolution == nil {
			oldSolution = make(map[string]float64)
		}
		for k, v := range solution {
			oldSolution[k] = v
		}
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

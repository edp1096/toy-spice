package analysis

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
)

type OperatingPoint struct{ BaseAnalysis }

func NewOP() *OperatingPoint {
	return &OperatingPoint{
		BaseAnalysis: *NewBaseAnalysis(),
	}
}

func (op *OperatingPoint) Setup(ckt *circuit.Circuit) error {
	op.Circuit = ckt
	return nil
}

func (op *OperatingPoint) doNRiter(gmin float64, maxIter int) error {
	var err error

	ckt := op.Circuit
	mat := ckt.GetMatrix()
	var oldSolution []float64
	cktStatus := &device.CircuitStatus{
		Time: 0,
		Mode: device.OperatingPointAnalysis,
		Temp: 300.15, // 27 = 300.15K
		Gmin: gmin,
	}

	for iter := range maxIter {
		mat.Clear()

		// First iteration have no previous solution so, skip
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
				reltol := op.convergence.reltol*math.Max(math.Abs(solution[i]), math.Abs(oldSolution[i])) + op.convergence.abstol

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

func (op *OperatingPoint) Execute() error {
	ckt := op.Circuit
	mat := ckt.GetMatrix()

	err := op.doNRiter(0, op.convergence.maxIter)
	if err == nil {
		solution := mat.Solution()
		op.storeResults(solution)

		return nil
	}

	numGminSteps := 10
	startGmin := float64(mat.Size) * 0.001
	gmin := startGmin * math.Pow(10, float64(numGminSteps))

	for i := 0; i <= numGminSteps; i++ {
		err := op.doNRiter(gmin, op.convergence.maxIter)
		if err != nil {
			return fmt.Errorf("gmin stepping failed at %g: %v", gmin, err)
		}
		gmin /= 10
	}

	err = op.doNRiter(0, op.convergence.maxIter)
	if err != nil {
		return fmt.Errorf("final solution failed with zero gmin: %v", err)
	}

	solution := mat.Solution()
	op.storeResults(solution)

	return nil
}

func (op *OperatingPoint) storeResults(solution []float64) {
	// Node voltage
	for nodeName, nodeIdx := range op.Circuit.GetNodeMap() {
		if nodeIdx > 0 {
			key := fmt.Sprintf("V(%s)", nodeName)
			op.results[key] = []float64{solution[nodeIdx]}
		}
	}
	// Branch current
	for devName, branchIdx := range op.Circuit.GetBranchMap() {
		key := fmt.Sprintf("I(%s)", devName)
		op.results[key] = []float64{solution[branchIdx]}
	}
}

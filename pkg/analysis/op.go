package analysis

import (
	"fmt"
	"math"

	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
	"github.com/edp1096/toy-spice/pkg/matrix"
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

func (op *OperatingPoint) doNRiter(gmin float64, maxIter int, initialSolution []float64) error {
	var err error
	ckt := op.Circuit
	mat := ckt.GetMatrix()
	var oldSolution []float64

	if initialSolution != nil {
		oldSolution = make([]float64, len(initialSolution))
		copy(oldSolution, initialSolution)
	} else {
		oldSolution = make([]float64, mat.Size+1)
	}

	ckt.Status = &device.CircuitStatus{
		Time: 0,
		Mode: device.OperatingPointAnalysis,
		Temp: 300.15, // 27 = 300.15K
		Gmin: gmin,
	}

	for iter := range maxIter {
		mat.Clear()

		err = ckt.UpdateNonlinearVoltages(oldSolution)
		if err != nil {
			return fmt.Errorf("updating nonlinear voltages: %v", err)
		}

		err = ckt.Stamp(ckt.Status)
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

		copy(oldSolution, solution)
	}

	return fmt.Errorf("failed to converge in %d iterations", maxIter)
}

func (op *OperatingPoint) calculateInitialEstimate() []float64 {
	ckt := op.Circuit
	size := ckt.GetMatrix().Size

	initialMatrix := matrix.NewMatrix(size, false)

	for _, dev := range ckt.GetDevices() {
		if _, isNonlinear := dev.(device.NonLinear); !isNonlinear {
			fmt.Println("linear device:", dev.GetName())
			dev.Stamp(initialMatrix, ckt.Status)
		}
	}

	err := initialMatrix.Solve()
	if err != nil {
		fmt.Println("failed to calculate initial estimate:", err)
		return nil
	}

	result := initialMatrix.Solution()
	return result
}

func (op *OperatingPoint) performSourceStepping() error {
	ckt := op.Circuit
	mat := ckt.GetMatrix()

	originalSources := make(map[string]float64)
	for _, dev := range ckt.GetDevices() {
		if v, ok := dev.(*device.VoltageSource); ok {
			originalSources[v.GetName()] = v.GetValue()
			v.SetValue(v.GetValue() * 0.1)
		}
	}

	defer func() {
		for name, origValue := range originalSources {
			for _, dev := range ckt.GetDevices() {
				if dev.GetName() == name {
					if v, ok := dev.(*device.VoltageSource); ok {
						v.SetValue(origValue)
					}
				}
			}
		}
	}()

	initialSolution := op.calculateInitialEstimate()
	var currentSolution []float64

	if initialSolution != nil {
		currentSolution = initialSolution
	} else {
		currentSolution = make([]float64, mat.Size+1)
	}

	// Increase 10% -> 100%
	for factor := 0.1; factor <= 1.0; factor += 0.1 {
		fmt.Printf("Source stepping: %.0f%%\n", factor*100)

		for name, origValue := range originalSources {
			for _, dev := range ckt.GetDevices() {
				if dev.GetName() == name {
					if v, ok := dev.(*device.VoltageSource); ok {
						v.SetValue(origValue * factor)
					}
				}
			}
		}

		err := op.doNRiter(0, op.convergence.maxIter, currentSolution)
		if err != nil {
			return fmt.Errorf("source stepping failed at %.0f%%: %v", factor*100, err)
		}

		currentSolution = mat.Solution()
	}

	return nil
}

func (op *OperatingPoint) Execute() error {
	ckt := op.Circuit
	mat := ckt.GetMatrix()

	// 선형 소자만으로 초기 추정값 계산
	initialSolution := op.calculateInitialEstimate()
	if initialSolution != nil {
		err := ckt.UpdateNonlinearVoltages(initialSolution)
		if err != nil {
			fmt.Println("Warning: Error updating nonlinear voltages:", err)
		}
	}

	// 초기 해를 doNRiter에 전달하여 Newton-Raphson 수행
	err := op.doNRiter(0, op.convergence.maxIter, initialSolution)
	if err == nil {
		solution := mat.Solution()
		op.storeResults(solution)
		return nil
	}

	fmt.Println("Newton-Raphson failed, trying Gmin stepping...", err)
	numGminSteps := 10
	startGmin := float64(mat.Size) * 0.001
	gmin := startGmin * math.Pow(10, float64(numGminSteps))

	// 현재 솔루션을 가져와서 Gmin stepping에 사용
	currentSolution := mat.Solution()

	for i := 0; i <= numGminSteps; i++ {
		err := op.doNRiter(gmin, op.convergence.maxIter, currentSolution)
		if err != nil {
			break
		}
		currentSolution = mat.Solution() // 다음 반복에 사용할 솔루션 업데이트
		gmin /= 10
	}

	err = op.doNRiter(0, op.convergence.maxIter, currentSolution)
	if err == nil {
		solution := mat.Solution()
		op.storeResults(solution)
		return nil
	}

	fmt.Println("Gmin stepping failed, performing source stepping...", err)
	err = op.performSourceStepping()
	if err != nil {
		return fmt.Errorf("source stepping failed: %v", err)
	}

	// Source stepping 후의 솔루션으로 최종 시도
	finalSolution := mat.Solution()
	err = op.doNRiter(0, op.convergence.maxIter, finalSolution)
	if err != nil {
		return fmt.Errorf("final solution failed: %v", err)
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

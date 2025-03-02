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
	ckt.Status = &device.CircuitStatus{
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

		if oldSolution == nil {
			oldSolution = make([]float64, len(solution))
		}
		copy(oldSolution, solution)
	}

	return fmt.Errorf("failed to converge in %d iterations", maxIter)
}

func (op *OperatingPoint) calculateInitialEstimate() []float64 {
	ckt := op.Circuit
	nodeMap := ckt.GetNodeMap()
	branchMap := ckt.GetBranchMap()
	size := len(nodeMap) + len(branchMap)

	initialSolution := make([]float64, size+1)

	for _, dev := range ckt.GetDevices() {
		if v, ok := dev.(*device.VoltageSource); ok {
			nodes := v.GetNodes()
			value := v.GetValue()

			if nodes[0] == 0 && nodes[1] > 0 {
				initialSolution[nodes[1]] = value
			} else if nodes[1] == 0 && nodes[0] > 0 {
				initialSolution[nodes[0]] = value
			}
		}
	}

	for _, dev := range ckt.GetDevices() {
		if bjt, ok := dev.(*device.Bjt); ok {
			nc := bjt.GetNodes()[0]
			nb := bjt.GetNodes()[1]
			ne := bjt.GetNodes()[2]

			if ne > 0 {
				initialSolution[ne] = 0.2
			}
			if nb > 0 {
				initialSolution[nb] = initialSolution[ne] + 0.7
			}
			if nc > 0 {
				vcc := 12.0
				for _, voltage := range initialSolution {
					if voltage > vcc/2 {
						vcc = voltage
					}
				}
				initialSolution[nc] = vcc / 2
			}
		}
	}

	return initialSolution
}

func (op *OperatingPoint) performSourceStepping() error {
	ckt := op.Circuit

	originalSources := make(map[string]float64)
	for _, dev := range ckt.GetDevices() {
		if v, ok := dev.(*device.VoltageSource); ok {
			originalSources[v.GetName()] = v.GetValue()
			v.SetValue(v.GetValue() * 0.05)
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

	steps := []float64{0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.85, 1.0}

	for _, factor := range steps {
		for name, origValue := range originalSources {
			for _, dev := range ckt.GetDevices() {
				if dev.GetName() == name {
					if v, ok := dev.(*device.VoltageSource); ok {
						v.SetValue(origValue * factor)
					}
				}
			}
		}

		initialSolution := op.calculateInitialEstimate()
		if initialSolution != nil {
			ckt.UpdateNonlinearVoltages(initialSolution)
		}

		err := op.doNRiter(0, op.convergence.maxIter)
		if err != nil {
			continue
		}
	}

	return nil
}

func (op *OperatingPoint) ExecuteNotUse() error {
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

func (op *OperatingPoint) Execute() error {
	ckt := op.Circuit
	mat := ckt.GetMatrix()

	initialSolution := op.calculateInitialEstimate()
	if initialSolution != nil {
		err := ckt.UpdateNonlinearVoltages(initialSolution)
		if err != nil {
			fmt.Println("Warning: Error updating nonlinear voltages:", err)
		}
	}

	err := op.doNRiter(0, op.convergence.maxIter)
	if err == nil {
		solution := mat.Solution()
		op.storeResults(solution)
		return nil
	}

	fmt.Println("Newton-Raphson failed, trying Gmin stepping...")
	numGminSteps := 10
	startGmin := float64(mat.Size) * 0.001
	gmin := startGmin * math.Pow(10, float64(numGminSteps))

	for i := 0; i <= numGminSteps; i++ {
		err := op.doNRiter(gmin, op.convergence.maxIter)
		if err != nil {
			break
		}
		gmin /= 10
	}

	err = op.doNRiter(0, op.convergence.maxIter)
	if err == nil {
		solution := mat.Solution()
		op.storeResults(solution)
		return nil
	}

	fmt.Println("Gmin stepping failed, performing source stepping...")
	err = op.performSourceStepping()
	if err != nil {
		return fmt.Errorf("source stepping failed: %v", err)
	}

	err = op.doNRiter(0, op.convergence.maxIter)
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

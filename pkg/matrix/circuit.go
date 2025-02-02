package matrix

import (
	"fmt"

	"github.com/edp1096/sparse"
)

type CircuitMatrix struct {
	Size         int
	matrix       *sparse.Matrix
	rhs          []float64
	rhsImag      []float64
	solution     []float64
	solutionImag []float64
	isComplex    bool
	config       *sparse.Configuration
}

func NewMatrix(size int, isComplex bool) *CircuitMatrix {
	separatedComplexVectors := false
	translate := false

	config := &sparse.Configuration{
		Real:                    true,
		Complex:                 isComplex,
		SeparatedComplexVectors: separatedComplexVectors,
		Expandable:              true,
		Translate:               translate,
		ModifiedNodal:           true,
		TiesMultiplier:          5,
		PrinterWidth:            140,
		Annotate:                0,
	}

	mat, err := sparse.Create(int64(size), config)
	if err != nil {
		fmt.Printf("Error creating sparse matrix: %v\n", err)
		return nil
	}

	vectorSize := size + 1 // rhs, solution size
	vectorSizeImag := size + 1
	if isComplex && !config.SeparatedComplexVectors {
		vectorSize *= 2
		vectorSizeImag = 1
	}

	return &CircuitMatrix{
		Size:         size,
		matrix:       mat,
		rhs:          make([]float64, vectorSize), // 1-based indexing
		rhsImag:      make([]float64, vectorSizeImag),
		solution:     make([]float64, vectorSize),
		solutionImag: make([]float64, vectorSizeImag),
		config:       config,
	}
}

func (m *CircuitMatrix) SetupElements() {
	for i := 1; i <= m.Size; i++ {
		for j := 1; j <= m.Size; j++ {
			m.matrix.GetElement(int64(i), int64(j))
		}
	}
}

func (m *CircuitMatrix) AddElement(i, j int, value float64) {
	if i <= 0 || j <= 0 || i > m.Size || j > m.Size {
		fmt.Printf("Warning: Matrix index out of bounds (i=%d, j=%d, size=%d)\n", i, j, m.Size)
		return
	}
	m.matrix.GetElement(int64(i), int64(j)).Real += value
}

func (m *CircuitMatrix) AddComplexElement(i, j int, real, imag float64) {
	if i <= 0 || j <= 0 || i > m.Size || j > m.Size {
		fmt.Printf("Warning: Matrix index out of bounds (i=%d, j=%d, size=%d)\n", i, j, m.Size)
		return
	}

	element := m.matrix.GetElement(int64(i), int64(j))
	element.Real += real
	element.Imag += imag
}

func (m *CircuitMatrix) AddComplexRHS(i int, real, imag float64) {
	if i <= 0 || i > m.Size {
		fmt.Printf("Warning: RHS index out of bounds (i=%d, size=%d)\n", i, m.Size)
		return
	}

	if m.config.SeparatedComplexVectors {
		m.rhs[i] += real
		m.rhsImag[i] += imag
	} else {
		m.rhs[2*i] += real
		m.rhs[2*i+1] += imag
	}
}

func (m *CircuitMatrix) AddRHS(i int, value float64) {
	if i <= 0 || i > m.Size {
		fmt.Printf("Warning: RHS index out of bounds (i=%d, size=%d)\n", i, m.Size)
		return
	}
	m.rhs[i] += value
}

func (m *CircuitMatrix) LoadGmin(gmin float64) {
	size := m.Size
	for i := 1; i <= size; i++ {
		if diag := m.GetDiagElement(i); diag != nil {
			diag.Real += gmin
		}
	}
}

func (m *CircuitMatrix) Clear() {
	m.matrix.Clear()
	for i := range m.rhs {
		m.rhs[i] = 0
	}
	for i := range m.rhsImag {
		m.rhsImag[i] = 0
	}
}

func (m *CircuitMatrix) Solve() error {
	var err error

	err = m.matrix.Factor()
	if err != nil {
		return fmt.Errorf("matrix factorization failed: %v", err)
	}

	if m.config.Complex {
		m.solution, m.solutionImag, err = m.matrix.SolveComplex(m.rhs, m.rhsImag)
	} else {
		m.solution, err = m.matrix.Solve(m.rhs)
	}

	if err != nil {
		return fmt.Errorf("matrix solve failed: %v", err)
	}

	return nil
}

func (m *CircuitMatrix) GetDiagElement(i int) *sparse.Element {
	if i <= 0 || i > m.Size {
		fmt.Printf("Warning: Diagonal index out of bounds (i=%d, size=%d)\n", i, m.Size)
		return nil
	}
	return m.matrix.Diags[i]
}

func (m *CircuitMatrix) RHS() []float64 {
	return m.rhs
}

func (m *CircuitMatrix) Solution() []float64 {
	return m.solution
}

func (m *CircuitMatrix) GetComplexSolution(i int) (float64, float64) {
	if !m.config.Complex || i <= 0 || i > m.Size {
		return 0, 0
	}
	return m.solution[i], m.solution[i+m.Size]
}

func (m *CircuitMatrix) SolutionImag() []float64 {
	return m.solutionImag
}

func (m *CircuitMatrix) PrintSystem() {
	fmt.Printf("\nCircuit Equations (%dx%d):\n", m.Size, m.Size)
	fmt.Println("Node equations 1..n, followed by branch equations")

	for i := 1; i <= m.Size; i++ {
		fmt.Printf("Equation %d:\n", i)
		rowHasElements := false
		for j := 1; j <= m.Size; j++ {
			element := m.matrix.GetElement(int64(i), int64(j))
			if m.config.Complex {
				if element.Real != 0 || element.Imag != 0 {
					if element.Imag == 0 {
						fmt.Printf("  %+g*x%d ", element.Real, j)
					} else {
						fmt.Printf("  (%g + j%g)*x%d ", element.Real, element.Imag, j)
					}
					rowHasElements = true
				}
			} else {
				if element.Real != 0 {
					fmt.Printf("  %+g*x%d ", element.Real, j)
					rowHasElements = true
				}
			}
		}
		if rowHasElements {
			if !m.config.Complex {
				fmt.Printf(" = %g\n", m.rhs[i])
			} else {
				if !m.config.SeparatedComplexVectors {
					fmt.Printf(" = %g + j%g\n", m.rhs[i], m.rhs[i+m.Size])
				} else {
					fmt.Printf(" = %g + j%g\n", m.rhs[i], m.rhsImag[i])
				}
			}
		}
	}

	m.matrix.Print(false, true, true)

	fmt.Printf("RHS:\n")
	for i := 1; i <= m.Size; i++ {
		if !m.config.Complex {
			fmt.Printf("  x%d = %g\n", i, m.rhs[i])
		} else {
			if !m.config.SeparatedComplexVectors {
				fmt.Printf("  x%d = %g + j%g\n", i, m.rhs[i], m.rhs[i+m.Size])
			} else {
				fmt.Printf("  x%d = %g + j%g\n", i, m.rhs[i], m.rhsImag[i])
			}
		}
	}
}

func (m *CircuitMatrix) printMatrixSummary() {
	fmt.Println("\nMATRIX SUMMARY")
	fmt.Printf("Size of matrix = %d x %d\n", m.Size, m.Size)

	maxElement := 0.0
	minElement := 1.79e+308
	elementCount := 0
	maxPivot := 0.0
	minPivot := 1.79e+308

	fmt.Println("Matrix before factorization:")
	fmt.Printf("%3s", "")
	for j := 1; j <= m.Size; j++ {
		fmt.Printf("%10d", j)
	}
	fmt.Println()

	for i := 1; i <= m.Size; i++ {
		fmt.Printf("%4d", i)
		for j := 1; j <= m.Size; j++ {
			value := m.matrix.GetElement(int64(i), int64(j)).Real
			fmt.Printf("%10.3f", value)

			if value != 0 {
				elementCount++
				if value > maxElement {
					maxElement = value
				}
				if value < minElement {
					minElement = value
				}
				if i == j && value > maxPivot {
					maxPivot = value
				}
				if i == j && value < minPivot {
					minPivot = value
				}
			}
		}
		fmt.Println()
	}

	fmt.Printf("Largest element in matrix = %.3f\n", maxElement)
	fmt.Printf("Smallest element in matrix = %.3f\n", minElement)
	fmt.Printf("Largest pivot element = %.3f\n", maxPivot)
	fmt.Printf("Smallest pivot element = %.3f\n", minPivot)
	fmt.Printf("Density = %.2f%%\n", float64(elementCount)*100/float64(m.Size*m.Size))
	fmt.Println()
}

func (m *CircuitMatrix) Destroy() {
	if m.matrix != nil {
		m.matrix.Destroy()
	}
}

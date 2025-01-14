package matrix

type DeviceMatrix interface {
	AddElement(i, j int, value float64) // 1-based indexing
	AddRHS(i int, value float64)
	AddComplexElement(i, j int, real, imag float64)
	AddComplexRHS(i int, real, imag float64)
}

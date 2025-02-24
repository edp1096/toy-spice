package util

type IntegrationMethod int

const (
	GearMethod IntegrationMethod = iota
	TrapezoidalMethod
)

type BackwardDifferentialFormula struct {
	coefficients []float64
	beta         float64
}

var BdfCoefficients = [6]BackwardDifferentialFormula{
	{[]float64{1.0}, 1.0},
	{[]float64{4.0 / 3.0, -1.0 / 3.0}, 2.0 / 3.0},
	{[]float64{18.0 / 11.0, -9.0 / 11.0, 2.0 / 11.0}, 6.0 / 11.0},
	{[]float64{48.0 / 25.0, -36.0 / 25.0, 16.0 / 25.0, -3.0 / 25.0}, 12.0 / 25.0},
	{[]float64{300.0 / 137.0, -300.0 / 137.0, 200.0 / 137.0, -75.0 / 137.0, 12.0 / 137.0}, 60.0 / 137.0},
	{[]float64{360.0 / 147.0, -450.0 / 147.0, 400.0 / 147.0, -225.0 / 147.0, 72.0 / 147.0, -10.0 / 147.0}, 60.0 / 147.0},
}

func GetIntegratorCoeffs(method IntegrationMethod, order int, dt float64) []float64 {
	switch method {
	case TrapezoidalMethod:
		return GetTrapezoidalCoeffs(order, dt)
	default:
		return GetBDFcoeffs(order, dt)
	}
}

func GetBDFcoeffs(order int, dt float64) []float64 {
	if order < 1 || order > 6 {
		order = 1
	}

	bdf := BdfCoefficients[order-1]
	coeffs := make([]float64, order+1)
	scale := 1.0 / (bdf.beta * dt)
	coeffs[0] = scale

	for i := 1; i <= order; i++ {
		coeffs[i] = -bdf.coefficients[i-1] * scale
	}

	return coeffs
}

func GetTrapezoidalCoeffs(order int, dt float64) []float64 {
	if order < 1 || order > 2 {
		order = 1
	}

	coeffs := make([]float64, 1)
	coeffs[0] = 2.0 / dt
	if order == 1 {
		coeffs[0] = 1.0 / dt
	}

	return coeffs
}

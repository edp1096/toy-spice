package util

import (
	"fmt"
	"math"
)

func FormatValueFactor(value float64, unit string) string {
	absValue := math.Abs(value)
	switch {
	case absValue >= 1:
		return fmt.Sprintf("%.3f %s", value, unit)
	case absValue >= 1e-3:
		return fmt.Sprintf("%.3f m%s", value*1e3, unit)
	case absValue >= 1e-6:
		return fmt.Sprintf("%.3f u%s", value*1e6, unit)
	case absValue >= 1e-9:
		return fmt.Sprintf("%.3f n%s", value*1e9, unit)
	case absValue >= 1e-12:
		return fmt.Sprintf("%.3f p%s", value*1e12, unit)
	default:
		return fmt.Sprintf("%.3e %s", value, unit)
	}
}

func FormatFrequency(freq float64) string {
	switch {
	case freq >= 1e6:
		return fmt.Sprintf("%7.3f MHz", freq/1e6)
	case freq >= 1e3:
		return fmt.Sprintf("%7.3f kHz", freq/1e3)
	default:
		return fmt.Sprintf("%7.3f Hz ", freq)
	}
}

func FormatMagnitudePhase(name string, value, phase float64) string {
	var magStr string
	if value >= 1000 {
		magStr = fmt.Sprintf("%8.2e", value) // e.g., "1.00e+03"
	} else if value < 0.001 {
		magStr = fmt.Sprintf("%8.2e", value) // e.g., "5.43e-05"
	} else {
		magStr = fmt.Sprintf("%8.3g", value) // e.g., "  732.5 "
	}
	phaseStr := fmt.Sprintf("%6.1f", phase) // e.g., "  90.0"
	return fmt.Sprintf("%s=%s<%sdeg", name, magStr, phaseStr)
}

func FormatMagnitude(value float64) string {
	if value >= 1000 || (value < 0.001 && value != 0) {
		return fmt.Sprintf("%8.2e", value) // "1.00e+03" or "5.43e-05"
	}
	return fmt.Sprintf("%8.3g", value) // "  732.5 "
}

func FormatPhase(value float64) string {
	return fmt.Sprintf("%6.1f", value) // "  90.0"
}

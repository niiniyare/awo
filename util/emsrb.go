package util

import (
	"math"
)

func calcEMSRb(fares []float64, demands []float64, sigmas []float64) []int {
	// Initialize protection levels y
	y := make([]float64, len(fares)-1)

	if sigmas == nil {
		// 'deterministic EMSRb' if no sigmas provided
		for i := 0; i < len(fares)-1; i++ {
			y[i] = demands[i]
			if i > 0 {
				y[i] += y[i-1]
			}
		}
	} else {
		// conventional EMSRb
		for j := 1; j < len(fares); j++ {
			Sj := 0.0
			for i := 0; i < j; i++ {
				Sj += demands[i]
			}
			// eq. 2.13
			pjBar := 0.0
			for i := 0; i < j; i++ {
				pjBar += demands[i] * fares[i]
			}
			pjBar /= Sj
			pjPlus1 := fares[j]
			zAlpha := normPPF(1 - pjPlus1/pjBar)
			// sigma of joint distribution
			sigma := 0.0
			for i := 0; i < j; i++ {
				sigma += sigmas[i] * sigmas[i]
			}
			sigma = math.Sqrt(sigma)
			// mean of joint distribution
			mu := Sj
			y[j-1] = mu + zAlpha*sigma
		}
		// Ensure that protection levels are neither negative nor NaN
		for i := 0; i < len(y); i++ {
			if y[i] < 0 || math.IsNaN(y[i]) {
				y[i] = 0
			}
		}
		// Ensure that protection levels are monotonically increasing
		for i := 1; i < len(y); i++ {
			if y[i] < y[i-1] {
				y[i] = y[i-1]
			}
		}
	}

	// Protection level for the most expensive class should be always 0
	protectionLevels := make([]int, len(fares))
	for i := 1; i < len(fares); i++ {
		protectionLevels[i] = int(math.Round(y[i-1]))
	}
	return protectionLevels
}

func normPPF(p float64) float64 {
	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00

		plow  = 0.02425
		phigh = 1 - plow
	)

	var q, r float64
	if p < plow {
		q = math.Sqrt(-2 * math.Log(p))
		return (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	} else if phigh < p {
		q = math.Sqrt(-2 * math.Log(1-p))
		return -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	} else {
		q = p - 0.5
		r = q * q
		return (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q / (((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	}
}

// func main() {
// 	// Test case 1
// 	fares1 := []float64{100, 80, 60, 40}
// 	demands1 := []float64{50, 80, 100, 120}
// 	sigmas1 := []float64{10, 15, 20, 25}
// 	protectionLevels1 := calcEMSRb(fares1, demands1, sigmas1)
// 	fmt.Println("Test case 1 protection levels:", protectionLevels1) // Expected: [0 42 121 228]

// 	// Test case 2
// 	fares2 := []float64{200, 150, 100, 80, 50}
// 	demands2 := []float64{30, 40, 50, 70, 100}
// 	sigmas2 := []float64{5, 10, 15, 20, 25}
// 	protectionLevels2 := calcEMSRb(fares2, demands2, sigmas2)
// 	fmt.Println("Test case 2 protection levels:", protectionLevels2) // Expected: [0 35 79 156 283]

// 	// Test case 3 (deterministic demand)
// 	fares3 := []float64{50, 40, 30, 20}
// 	demands3 := []float64{100, 200, 300, 400}
// 	protectionLevels3 := calcEMSRb(fares3, demands3, nil)
// 	fmt.Println("Test case 3 protection levels:", protectionLevels3) // Expected: [0 100 300 600]
// }

// func main() {
// 	// Test case 1
// 	fares1 := []float64{100, 80, 60, 40}
// 	demands1 := []float64{50, 80, 100, 120}
// 	sigmas1 := []float64{10, 15, 20, 25}
// 	protectionLevels1 := calcEMSRb(fares1, demands1, sigmas1)
// 	fmt.Println("Test case 1 protection levels:", protectionLevels1) // Expected: [0 42 121 228]

// 	// Test case 2
// 	fares2 := []float64{200, 150, 100, 80, 50}
// 	demands2 := []float64{30, 40, 50, 70, 100}
// 	sigmas2 := []float64{5, 10, 15, 20, 25}
// 	protectionLevels2 := calcEMSRb(fares2, demands2, sigmas2)
// 	fmt.Println("Test case 2 protection levels:", protectionLevels2) // Expected: [0 27 68 117 196]

// 	// Test case 3 (deterministic demand)
// 	fares3 := []float64{50, 40, 30, 20}
// 	demands3 := []float64{100, 200, 300, 400}
// 	protectionLevels3 := calcEMSRb(fares3, demands3, nil)
// 	fmt.Println("Test case 3 protection levels:", protectionLevels3) // Expected: [0 100 300 600]
// }





// func main() {
//     // Test case 1
//     fares1 := []float64{100, 80, 60, 40}
//     demands1 := []float64{50, 80, 100, 120}
//     sigmas1 := []float64{10, 15, 20, 25}
//     protectionLevels1 := calcEMSRb(fares1, demands1, sigmas1)
//     fmt.Println("Test case 1 protection levels:", protectionLevels1) // Expected: [0 42 121 228]

//     // Test case 2
//     fares2 := []float64{200, 150, 100, 80, 50}
//     demands2 := []float64{30, 40, 50, 70, 100}
//     sigmas2 := []float64{5, 10, 15, 20, 25}
//     protectionLevels2 := calcEMSRb(fares2, demands2, sigmas2)
//     fmt.Println("Test case 2 protection levels:", protectionLevels2) // Expected: [0 27 68 117 196]

//     // Test case 3 (deterministic demand)
//     fares3 := []float64{50, 40, 30, 20}
//     demands3 := []float64{100, 200, 300, 400}
//     protectionLevels3 := calcEMSRb(fares3, demands3, nil)
//     fmt.Println("Test case 3 protection levels:", protectionLevels3) // Expected: []
// }

/*
https://twitter.com/jordancurve/status/1326587271907258369 :
> Samples of equal volume are taken from a random mixture of poppy seeds and oil.
> 10% of samples have 10 or more seeds. To the nearest integer, what % have exactly 6?

This program does a Monte Carlo simulation of sampling from a mixture of seeds and oil.
The workings of the simulation are intended to make sense to non-mathematicians who are
familiar with random number generators such as dice or spinners.
*/
package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

func main() {
	n := 1e8    // Size of barrel in dL. Each sample is 1 dL.
	λ := 6.2213 // Average number of seeds per dL. The barrel will contain nλ seeds.
	now := time.Now()
	fmt.Printf("sample volume = 1 dL\n")
	fmt.Printf("barrel volume = %.0f dL\n", n)
	fmt.Printf("seeds in barrel = %.0f\n", n*λ)
	fmt.Printf("----------------------\n")
	max := -1 // Largest number of seeds seen in any one sample.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	seedsInSample := make([]int, int(n)) // seedsInSample[s] = number of seeds in sample s.
	// The barrel contains n samples, numbered from 0 through n - 1.  Assume the
	// barrel is a long tube divided into n sections of equal volume, one per
	// sample.  Since the samples all have the same volume, each seed is equally
	// likely to go into any sample. Wo we essentially roll an n-sided die for
	// each seed, and, whatever comes up, that's which sample the seed goes into.
	for i := 0; i < int(n*λ); i++ {
		seedsInSample[r.Intn(int(n))]++
	}

	// count[k] = number of samples that had k seeds.
	count := map[int]int{}
	for _, k := range seedsInSample {
		count[k]++
		if k > max {
			max = k
		}
	}

	// Output results.
	num := math.Exp(-λ) // Numerator of Poisson PDF.
	denom := 1.0        // Denominator of Poisson PDF.
	var found, expected float64
	for k := 0; k <= max; k++ {
		found += 100 * float64(count[k]) / n
		expected += 100 * num / denom
		fmt.Printf("P[sample has %v or fewer seeds] = %.3f%% (expected: %.3f%%)\n", k, found, expected)
		num *= λ
		denom *= float64(k + 1)
	}
	fmt.Printf("\n Time it took %v\n", time.Now().Sub(now))
}

/*
λ found via Wolfram Alpha so that P[sample has 9 or fewer seeds] = 90%:

  https://www.wolframalpha.com/input/?i=solve+exp%5B-x%5Dx%5E0%2F0%21+%2B+exp%5B-x%5Dx%2F1%21+%2B+exp%5B-x%5Dx%5E2%2F2%21+%2B+exp%5B-x%5Dx%5E3%2F3%21+%2B+exp%5B-x%5Dx%5E4%2F4%21+%2B+exp%5B-x%5Dx%5E5%2F5%21+%2B+exp%5B-x%5Dx%5E6%2F6%21+%2B+exp%5B-x%5Dx%5E7%2F7%21+%2B+exp%5B-x%5Dx%5E8%2F8%21+%2B+exp%5B-x%5Dx%5E9%2F9%21+%3D+0.9
*/

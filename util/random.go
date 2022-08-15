package util

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz"
const number = "1234567890"
const alphaNum = alphabet + number

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomInt generates a random integer between min and max
func RandomInt(min, max int64) int64 {
	return min + rand.Int63n(max-min+1)
}

// RandomString generates a random string of length n
func RandomString(n int) string {
	var sb strings.Builder
	k := len(alphabet)

	for i := 0; i < n; i++ {
		c := alphabet[rand.Intn(k)]
		sb.WriteByte(c)
	}

	return sb.String()
}

//RandomAlphaString generates a random string and numbers of length n
func RandomAlphaString(n int) string {
	var sb strings.Builder
	k := len(alphaNum)

	for i := 0; i < n; i++ {
		c := alphabet[rand.Intn(k)]
		sb.WriteByte(c)
	}

	return sb.String()
}

func RandomFloat64(min, max float64) float64 {
	return min + rand.Float64()*(max-min)

}

func RandomFloat32(min, max float32) float32 {
	return min + rand.Float32()*(max-min)
}

// RandomOwner generates a random owner name
func RandomOwner() string {
	return RandomString(6)
}

// RandomMoney generates a random amount of money
func RandomMoney() int64 {
	return RandomInt(0, 1000)
}

// RandomCurrency generates a random currency code
func RandomCurrency() string {
	currencies := []string{USD, EUR, CAD}
	n := len(currencies)
	return currencies[rand.Intn(n)]
}

// range from -90to90 for latitude and-180to180 for longitude.
func RandomPoint() string {
	latitude := RandomFloat32(-90, 90)
	longitude := RandomFloat32(-180, 180)
	return fmt.Sprintf("(%.6f %.6f)", longitude, latitude)
}

func RandomEmail() string {

	return fmt.Sprintf("%s@email.com", RandomString(6))
}

func RandomFlight() string {
	n := RandomInt(100, 999)
	s := strings.ToUpper(RandomString(2))

	return fmt.Sprintf("%v%v", s, n)
}
func RandomFlightNo(s string) string {
	n := RandomInt(100, 999)
	// s := strings.ToUpper(RandomString(2))

	return fmt.Sprintf("%v%v", s, n)
}

package moira

import (
	"io/ioutil"
	"math"
	"os"

	"github.com/satori/go.uuid"
)

const (
	maxUint = ^uint(0)
	minUint = 0
	maxInt  = int(maxUint >> 1)
	minInt  = -maxInt - 1
)

// GetFileContent reads file and returns its content as string
func GetFileContent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	return string(bytes), err
}

func MaxI(args ...int) int {
	result := minInt
	for _, arg := range args {
		if arg > result {
			result = arg
		}
	}
	return result
}

func MaxI32(args ...int32) int32 {
	result := int32(math.MinInt32)
	for _, arg := range args {
		if arg > result {
			result = arg
		}
	}
	return result
}

func MaxI64(args ...int64) int64 {
	result := int64(math.MinInt64)
	for _, arg := range args {
		if arg > result {
			result = arg
		}
	}
	return result
}

func MinI(args ...int) int {
	result := maxInt
	for _, arg := range args {
		if arg < result {
			result = arg
		}
	}
	return result
}

func MinI32(args ...int32) int32 {
	result := int32(math.MaxInt32)
	for _, arg := range args {
		if arg < result {
			result = arg
		}
	}
	return result
}

func MinI64(args ...int64) int64 {
	result := int64(math.MaxInt64)
	for _, arg := range args {
		if arg < result {
			result = arg
		}
	}
	return result
}

func NewStrID() string {
	return uuid.NewV4().String()
}

// UseString gets pointer value of string or default string if pointer is nil
func UseString(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

// UseFloat64 gets pointer value of float64 or default float64 if pointer is nil
func UseFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

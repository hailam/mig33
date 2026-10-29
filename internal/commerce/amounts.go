package commerce

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/faults"
)

func FormatAmount(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	return fmt.Sprintf("%s%d.%02d", sign, n/100, n%100)
}

func ParseAmount(text string) (int64, error) {
	if len(text) == 0 || len(text) > 12 {
		return 0, faults.ErrInvalid
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 {
		return 0, faults.ErrInvalid
	}
	for _, part := range parts {
		if part == "" {
			return 0, faults.ErrInvalid
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return 0, faults.ErrInvalid
			}
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 1000000 {
		return 0, faults.ErrInvalid
	}
	fraction := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, faults.ErrInvalid
		}
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		fraction, _ = strconv.ParseInt(f, 10, 64)
	}
	amount := whole*100 + fraction
	if amount <= 0 || amount > 100000000 {
		return 0, faults.ErrInvalid
	}
	return amount, nil
}

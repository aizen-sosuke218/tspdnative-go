package tspdnative_test

import (
	"context"
	"fmt"
	"log"

	"github.com/aizen-sosuke218/tspdnative-go"
)

// ExampleSolve shows the one-shot API. Replace the proxy with an empty string
// to connect directly.
func ExampleSolve() {
	result, err := tspdnative.Solve(context.Background(), tspdnative.Options{
		Proxy: "user:pass@host:8080",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.CookieString) // k=v; k2=v2; ...
	fmt.Println(result.Solved)       // true when TSPD_101 was issued
}

// Package tspdnative solves the F5 TSPD JavaScript challenge served by
// https://onlineservices.immigration.govt.nz/ without executing any
// JavaScript.
//
// The solver reproduces the sensor's offline pipeline:
//
//  1. GET the target page to obtain the bobcmn session configuration and the
//     hex challenge.
//  2. GET the referenced sensor script(s) to extract the per-build key.
//  3. Unseal the challenge, read the embedded 48-byte context, seal the
//     fingerprint and assemble the answer cookie.
//  4. GET the target again with the answer cookie; the edge responds with
//     TSPD_101 when the answer is accepted.
//
// The package depends only on the Go standard library. It does not embed a
// JavaScript engine and does not use cgo.
//
// # Scope
//
// The implementation targets the bobcmn layout and crypto parameters of the
// immigration host. Other TSPD deployments may use different layouts and are
// intentionally out of scope.
//
// # Quick start
//
//	result, err := tspdnative.Solve(ctx, tspdnative.Options{
//		Proxy: "user:pass@host:8080",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(result.CookieString)
package tspdnative

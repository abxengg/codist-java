package java

import (
	"strings"
	"testing"
)

const paymentController = `package com.shop.payment;

@RestController
@RequestMapping("/payments")
public class PaymentController {
    @PostMapping("/charge")
    public String charge(@RequestBody ChargeRequest r) { return "ok"; }

    @GetMapping("/{id}")
    public String get(@PathVariable long id) { return "ok"; }

    @RequestMapping("/refunds")
    public String refunds() { return "ok"; }
}
`

func clientFile(annotation, method string) string {
	return "package com.shop.order;\n\n" + annotation + "\npublic interface PaymentClient {\n    " + method + "\n    String call();\n}\n"
}

func TestUnservedCalls(t *testing.T) {
	for _, c := range []struct {
		name, annotation, method string
		want                     bool
	}{
		{"renamed route (shop-lab)", `@FeignClient(name = "payment-service", path = "/payments")`, `@PostMapping("/charges")`, true},
		{"served route", `@FeignClient(name = "payment-service", path = "/payments")`, `@PostMapping("/charge")`, false},
		{"path variable", `@FeignClient(name = "payment-service")`, `@GetMapping("/payments/{orderId}")`, false},
		{"any verb served", `@FeignClient(name = "payment-service", path = "/payments")`, `@DeleteMapping("/refunds")`, false},
		{"wrong verb", `@FeignClient(name = "payment-service", path = "/payments")`, `@PutMapping("/charge")`, true},
		// GET /payments/charge reaches GET /payments/{id}: served, whatever the id then does with "charge".
		{"a variable takes the segment", `@FeignClient(name = "payment-service", path = "/payments")`, `@GetMapping("/charge")`, false},
		{"service outside the repository", `@FeignClient(name = "stripe", url = "https://api.stripe.com")`, `@PostMapping("/v1/charges")`, false},
		{"value form", `@FeignClient("payment-service")`, `@PostMapping("/payments/charges")`, true},
	} {
		server := Parse("payment-service/src/main/java/com/shop/payment/PaymentController.java", []byte(paymentController))
		client := Parse("order-service/src/main/java/com/shop/order/PaymentClient.java", []byte(clientFile(c.annotation, c.method)))
		facts := FactsOf([]*File{server, client})
		var got []Issue
		for _, is := range CheckWith(client, nil, facts) {
			if is.Rule == "unserved-call" {
				got = append(got, is)
			}
		}
		if (len(got) > 0) != c.want {
			t.Errorf("%s: %+v", c.name, got)
		}
		if c.want && len(got) > 0 && (got[0].Line != 5 || !strings.Contains(got[0].Detail, "serves: ANY /payments/refunds, GET /payments/{id}, POST /payments/charge")) {
			t.Errorf("%s: line %d, detail %q", c.name, got[0].Line, got[0].Detail)
		}
	}
}

// Only the changed lines are checked: a client with an old unserved route is not reported for an unrelated change.
func TestUnservedCallsOnlyOnChangedLines(t *testing.T) {
	server := Parse("payment-service/src/main/java/com/shop/payment/PaymentController.java", []byte(paymentController))
	client := Parse("order-service/src/main/java/com/shop/order/PaymentClient.java",
		[]byte(clientFile(`@FeignClient(name = "payment-service", path = "/payments")`, `@PostMapping("/charges")`)))
	facts := FactsOf([]*File{server, client})
	for _, is := range CheckWith(client, map[int]bool{3: true}, facts) {
		if is.Rule == "unserved-call" {
			t.Errorf("reported off the changed lines: %+v", is)
		}
	}
}

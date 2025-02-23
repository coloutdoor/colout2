package main

// CalculateSalesTax applies WA sales tax to the subtotal (deck + rail costs).
// Currently hardcoded at 10% (6.5% state + 3.5% local, e.g., Seattle).
// Future: Replace with dynamic lookup based on address.
func CalculateSalesTax(subtotal float64) float64 {
	const taxRate = 0.087 // 8.7% total WA sales tax
	return subtotal * taxRate
}

An implementation of the receipt processor defined in the specification at https://github.com/fetch-rewards/receipt-processor-challenge.

# Requirements

* go 1.22.2
* github.com/google/uuid v1.6.0

# Usage Instructions (for local testing)

* Run receipt_processor with `go run`, e.g. `go run receipt_processor &` on Linux
* Send receipt JSON via POST to localhost:8080/receipts/process
    * Server will respond with a single-value JSON object specifying the randomly generated UUID associated with the receipt
    * E.g., a test can be made from the Linux command line with `curl -X POST http://localhost:8080/receipts/process -H "Content-Type: application/json" -d '{"retailer": "Walgreens","purchaseDate": "2022-01-02","purchaseTime": "08:13","total": "2.65","items": [{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},{"shortDescription": "Dasani", "price": "1.40"}]}'`
* Check receipt score via GET at localhost:8080/receipts/{the assigned UUID}/points
    * Server will respond with a single-value JSON object specifying the points allocated to the receipt with the associated UUID
    * E.g., a test might be made from the Linux command line with `curl http://localhost:8080/receipts/e2959510-d71b-4156-86a5-1abc87010070/points` for a receipt assigned the UUID e2959510-d71b-4156-86a5-1abc87010070

# Considerations

* This is my first time working with Go! I've tried to follow the rules of "idiomatic Go" as I've understood them through my self-guided internet crash course on the language, but I know there are areas where I've deviated. One such area is variable naming. As I understand it, the Go community heavily favors very terse, even single-letter variables. When it felt reasonable I've followed this convention, but in several places I felt that more descriptive names were much more helpful for understanding the function of the code.
* The webserver itself is set up with http.ListenAndServe. It doesn't allow for graceful termination, but I've used it in this case because (as per specification) this receipt processor holds all information in memory. Without any persistent information, it doesn't seem to me like there's much value in graceful termination.
* I made the decision to have two pairs of structs, RawItem/RawReceipt and item/receipt, rather than just one. Having the first pair, with fields exactly matching the API, seemed necessary in order to use Go's standard JSON unmarshalling tools. However, the API indicated additional constraints for several string fields, and I wanted to enforce those constraints. Further, several scoring tasks would be performed more naturally if the price and date information were converted ahead of time to more appropriate types than string.
* It wasn't necessary to break each scoring rule out into its own function, but I preferred the modularity. If we imagine that in the future the scoring rules may change, new rules may be added, or old rules may be deleted, I think this approach is superior.
* Likewise, with the rules as implemented there's really no advantage to passing in the current score as an int pointer rather than just returning the difference in score, but I preferred the former since we can imagine adding rules in the future like "If the purchase was made on a Friday, increase all prior awards by 20%." It's not flexible enough to cover all situations, but I figured a little extra flexibility wouldn't hurt.
* I think that *maybe* the ideal move with these scoring functions would be to create an interface that unites them and then put all of those interfaces into a Go slice? Since I'm new to Go, I'm really not sure on this point. Definitely the sort of thing I would like to ask a collaborator about, in practice.
* The specification doesn't actually say anything about how IDs should be generated, but the example given implies that they're to be UUIDs. Using a pre-built solution for that seemed preferable despite the need for an external dependency.
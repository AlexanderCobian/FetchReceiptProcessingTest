/*

A webservice that processes and scores receipts according to the API at
https://github.com/fetch-rewards/receipt-processor-challenge

*/

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

/*
RawItem and RawReceipt structs exist to facilitate the unmarshalling
of JSON data from the incoming POST requests.
*/
type RawItem struct {
	ShortDescription string `json:"shortDescription"`
	Price            string `json:"price"`
}

type RawReceipt struct {
	Retailer     string    `json:"retailer"`
	PurchaseDate string    `json:"purchaseDate"`
	PurchaseTime string    `json:"purchaseTime"`
	Items        []RawItem `json:"items"`
	Total        string    `json:"total"`
}

/*
item and receipt structs exist to allow conversion of instances of the above
two structs into a format where the fields are of a moredirectly useful type.
Namely, Price and Total are converted to intsmeasured in cents and
PurchaseDate and PurchaseTime are combined into a single time.Time object
*/
type item struct {
	shortDescription string
	cents            int
}

type receipt struct {
	retailer         string
	purchaseDatetime time.Time
	items            []item
	cents            int
}

// Maps receipt UUIDs to the points that they earned on submission
var receiptPoints map[string]int = make(map[string]int)

// Implements this rule from the spec:
//
// One point for every alphanumeric character in the retailer name.
func scoreRetailerName(r receipt, oldScore *int) {
	for _, char := range r.retailer {
		if unicode.IsLetter(char) || unicode.IsNumber(char) {
			*oldScore += 1
		}
	}
}

// Implements this rule from the spec:
//
// 50 points if the total is a round dollar amount with no cents.
func scoreNoCentsBonus(r receipt, oldScore *int) {
	if r.cents%100 == 0 {
		*oldScore += 50
	}
}

// Implements this rule from the spec:
//
// 25 points if the total is a multiple of 0.25.
func scoreEvenQuarterBonus(r receipt, oldScore *int) {
	if r.cents%25 == 0 {
		*oldScore += 25
	}
}

// Implements this rule from the spec:
//
// 5 points for every two items on the receipt.
func scoreNumItems(r receipt, oldScore *int) {
	*oldScore += (len(r.items) / 2) * 5
}

// Implements this rule from the spec:
//
// If the trimmed length of the item description is a multiple of 3, multiply
// the price by 0.2 and round up to the nearest integer. The result is the
// number of points earned.
func scoreItemDescriptionLengths(r receipt, oldScore *int) {
	for _, item := range r.items {
		if len(strings.TrimSpace(item.shortDescription))%3 == 0 {
			*oldScore += int(math.Ceil(float64(item.cents) * 0.002))
		}
	}
}

// Implements this rule from the spec:
//
// 6 points if the day in the purchase date is odd.
func scoreOddPurchaseDates(r receipt, oldScore *int) {
	if r.purchaseDatetime.Day()%2 == 1 {
		*oldScore += 6
	}
}

// Implements this rule from the spec:
//
// 10 points if the time of purchase is after 2:00pm and before 4:00pm.
func scoreAfternoonBonus(r receipt, oldScore *int) {
	const (
		twoPm  = 14 * 60
		fourPm = 16 * 60
	)
	purchaseTime := r.purchaseDatetime.Hour()*60 + r.purchaseDatetime.Minute()
	if twoPm < purchaseTime && purchaseTime < fourPm {
		*oldScore += 10
	}
}

// Handler for POST requests to /receipts/process
func processReceipt(w http.ResponseWriter, req *http.Request) {

	// Generate UUID that will correspond to this receipt's recorded points
	newId, _ := uuid.NewRandom()

	// Unpack the receipt JSON into rawReceipt
	var rawReceipt RawReceipt
	data, _ := io.ReadAll(req.Body)
	err := json.Unmarshal(data, &rawReceipt)
	// If parsing the JSON fails, send the client a 400
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "The receipt is invalid.")
		return
	}

	// Convert rawReceipt into validReceipt, and in so doing ensure that the
	// JSON meets additional API requirements. If it doesn't, send the client
	// a 400.
	validReceipt, validationSuccessful := validateAndConvertReceipt(rawReceipt)
	if !validationSuccessful {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "The receipt is invalid.")
		return
	}

	// Call each of the score functions and tally up the total score
	pointsEarned := 0
	scoreRetailerName(validReceipt, &pointsEarned)
	scoreNoCentsBonus(validReceipt, &pointsEarned)
	scoreEvenQuarterBonus(validReceipt, &pointsEarned)
	scoreNumItems(validReceipt, &pointsEarned)
	scoreItemDescriptionLengths(validReceipt, &pointsEarned)
	scoreOddPurchaseDates(validReceipt, &pointsEarned)
	scoreAfternoonBonus(validReceipt, &pointsEarned)

	// Save the receipt UUID and points as a key-value pair in pointsEarned
	receiptPoints[newId.String()] = pointsEarned

	// Return the UUID as a JSON object
	fmt.Fprintf(w, "{ \"id\": \"%v\" }", newId)

}

// Attempts to convert a RawReceipt into a receipt, validating API requirements
// along the way. Returned bool indicates success/failure.
func validateAndConvertReceipt(old RawReceipt) (receipt, bool) {

	// Prepare the regexes used to validate names, descriptions, and prices
	retailerAndDescRegex := regexp.MustCompile(`^[\w\s\-&]+$`)
	priceRegex := regexp.MustCompile(`^\d+\.\d{2}$`)

	var new receipt = receipt{}

	// Validate and copy over retailer name
	match := retailerAndDescRegex.MatchString(old.Retailer)
	if !match {
		return receipt{}, false
	}
	new.retailer = old.Retailer

	// Validate and copy over purchase date and time
	dateString := old.PurchaseDate + " " + old.PurchaseTime
	datetime, err := time.Parse("2006-01-02 15:04", dateString)
	if err != nil {
		fmt.Println(err)
		return receipt{}, false
	}
	new.purchaseDatetime = datetime

	// Validate and copy over the total price on the receipt
	match = priceRegex.MatchString(old.Total)
	if !match {
		return receipt{}, false
	}
	cents, _ := strconv.Atoi(old.Total[:len(old.Total)-3] + old.Total[len(old.Total)-2:])
	new.cents = cents

	// Enforce the rule that receipts must have at least one item
	if len(old.Items) == 0 {
		return receipt{}, false
	}

	for _, oldItem := range old.Items {
		newItem := item{}

		// Validate and copy over each item's description
		match = retailerAndDescRegex.MatchString(oldItem.ShortDescription)
		if !match {
			return receipt{}, false
		}
		newItem.shortDescription = oldItem.ShortDescription

		// Validate and copy over each item's price
		match = priceRegex.MatchString(oldItem.Price)
		if !match {
			return receipt{}, false
		}
		cents, _ = strconv.Atoi(oldItem.Price[:len(oldItem.Price)-3] + oldItem.Price[len(oldItem.Price)-2:])
		newItem.cents = cents

		new.items = append(new.items, newItem)
	}

	// If all validation succeeds, return the new receipt instance
	return new, true

}

// Handler for GET requests to /receipts/{id}/points
func getPoints(w http.ResponseWriter, req *http.Request) {

	id := strings.Split(req.URL.Path, "/")[2]

	if points, present := receiptPoints[id]; !present {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "No receipt found for that ID.")
	} else {
		fmt.Fprintf(w, "{ \"points\": %d }", points)
	}

}

// Sets up the handlers for the POST and GET requests and begins listening for
// said requests on port 8080
func main() {

	http.HandleFunc("POST /receipts/process", processReceipt)
	http.HandleFunc("GET /receipts/{id}/points", getPoints)

	http.ListenAndServe(":8080", nil)

}
